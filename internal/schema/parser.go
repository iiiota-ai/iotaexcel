// Package schema 负责把 xlsx 包读取出的原始字符串矩阵解析成强类型配置模型。
//
// 这里集中实现 Excel 规则校验：文件名/sheet 名/字段名标识符检查、前 4 行表头解析、
// 唯一 key 约束、字段用途别名、类型转换、默认值处理、空行跳过和 schemaHash 计算。
package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"iotaexcel/internal/constants"
	"iotaexcel/internal/model"
)

// Options 控制 schema 解析阶段的行为。
// Target 会影响 schemaHash，因为不同目标导出的字段集合可能不同；
// CheckRef 当前由 app 层统一延后处理，字段保留用于后续扩展。
type Options struct {
	Target   string
	CheckRef bool
}

// identRE 是文件名、sheet 名、字段名和 ref<T> 目标名共用的标识符规则。
// 当前采用 C#/Go 都容易接受的 ASCII 标识符子集，避免生成代码时出现语言相关转义。
var identRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ParseWorkbook 校验 Excel 文件级规则，并逐个解析 sheet。
// 文件名会映射为 C# 代码文件名，因此需要先通过标识符检查；任意 sheet 解析失败会使当前 workbook 失败。
func ParseWorkbook(raw model.RawWorkbook, opts Options) (model.Workbook, error) {
	base := strings.TrimSuffix(filepath.Base(raw.RelPath), filepath.Ext(raw.RelPath))
	if !identRE.MatchString(base) {
		return model.Workbook{}, fmt.Errorf("excel file name %q is not a valid identifier", base)
	}

	wb := model.Workbook{SourcePath: raw.SourcePath, RelPath: raw.RelPath}
	for _, rawSheet := range raw.Sheets {
		if !identRE.MatchString(rawSheet.Name) {
			return model.Workbook{}, fmt.Errorf("sheet name %q is not a valid identifier", rawSheet.Name)
		}
		sheet, err := parseSheet(rawSheet, opts)
		if err != nil {
			return model.Workbook{}, fmt.Errorf("%s: %w", rawSheet.Name, err)
		}
		wb.Sheets = append(wb.Sheets, sheet)
	}
	return wb, nil
}

// parseSheet 解析单个 sheet。
// 规则要求前 4 行分别是字段名、类型、用途、注释，第 5 行开始才是数据；
// 解析表头时会确定 fieldNo/wireType/key 字段，解析数据时会跳过全空行并检查 key 唯一性。
func parseSheet(raw model.RawSheet, opts Options) (model.Sheet, error) {
	if len(raw.Rows) < 5 {
		return model.Sheet{}, fmt.Errorf("sheet must contain at least 5 rows")
	}

	maxCols := maxColumns(raw.Rows[:4])
	fields := make([]model.Field, 0, maxCols)
	names := map[string]bool{}
	keyCount := 0
	keyColumn := -1

	for col := 0; col < maxCols; col++ {
		rawName := cellAt(raw.Rows[0], col)
		name, isKey, err := parseFieldName(rawName)
		if err != nil {
			return model.Sheet{}, fmt.Errorf("column %d field name: %w", col+1, err)
		}
		if names[name] {
			return model.Sheet{}, fmt.Errorf("duplicate field name %q", name)
		}
		names[name] = true

		typeSpec, err := ParseType(cellAt(raw.Rows[1], col))
		if err != nil {
			return model.Sheet{}, fmt.Errorf("field %s type: %w", name, err)
		}
		usage, err := ParseUsage(cellAt(raw.Rows[2], col))
		if err != nil {
			return model.Sheet{}, fmt.Errorf("field %s usage: %w", name, err)
		}
		if isKey {
			keyCount++
			keyColumn = col
			if usage == model.UsageComment {
				return model.Sheet{}, fmt.Errorf("key field %s cannot be comment", name)
			}
			if !isAllowedKeyType(typeSpec.Kind) {
				return model.Sheet{}, fmt.Errorf("key field %s type %s is not allowed", name, typeSpec.Raw)
			}
		}
		fields = append(fields, model.Field{
			Name:        name,
			RawName:     rawName,
			Type:        typeSpec,
			Usage:       usage,
			Comment:     cellAt(raw.Rows[3], col),
			IsKey:       isKey,
			ColumnIndex: col,
			FieldNo:     uint64(col + 1),
			WireType:    wireType(typeSpec.Kind),
			Binary:      usage != model.UsageComment,
		})
	}
	if keyCount == 0 {
		return model.Sheet{}, fmt.Errorf("missing unique key field")
	}
	if keyCount > 1 {
		return model.Sheet{}, fmt.Errorf("multiple key fields defined")
	}

	sheet := model.Sheet{Name: raw.Name, Fields: fields}
	keys := map[string]bool{}
	for i := 4; i < len(raw.Rows); i++ {
		row := raw.Rows[i]
		if isEmptyRow(row) {
			sheet.SkippedEmptyRows = append(sheet.SkippedEmptyRows, i+1)
			continue
		}

		key := strings.TrimSpace(cellAt(row, keyColumn))
		if key == "" {
			return model.Sheet{}, fmt.Errorf("row %d key is empty", i+1)
		}
		if keys[key] {
			return model.Sheet{}, fmt.Errorf("row %d duplicate key %q", i+1, key)
		}
		keys[key] = true

		parsed := model.Row{Index: i + 1, Key: key, Values: map[string]model.CellValue{}}
		for _, field := range fields {
			rawValue := cellAt(row, field.ColumnIndex)
			value, usedDefault, convErr := ConvertValue(rawValue, field.Type)
			cell := model.CellValue{Raw: rawValue, Value: value, Default: usedDefault}
			if convErr != nil {
				cell.Error = convErr.Error()
				sheet.ConversionErrors = append(sheet.ConversionErrors, fmt.Sprintf("row %d field %s: %v", i+1, field.Name, convErr))
			}
			if usedDefault {
				sheet.DefaultValueCount++
			}
			parsed.Values[field.Name] = cell
		}
		sheet.Rows = append(sheet.Rows, parsed)
	}
	sheet.SchemaHash = SchemaHash(sheet, opts.Target)
	return sheet, nil
}

// ParseType 把表头中的类型文本转换为 TypeSpec。
// 基础类型统一转小写；ref<T> 的 T 保留大小写，因为它必须精确匹配 sheet 名。
func ParseType(raw string) (model.TypeSpec, error) {
	trimmed := strings.TrimSpace(raw)
	text := strings.ToLower(trimmed)
	if text == "" {
		return model.TypeSpec{}, fmt.Errorf("type is empty")
	}
	switch text {
	case "bool", "int", "int32", "int64", "float", "double", "string", "bytes", "datetime":
		return model.TypeSpec{Raw: text, Kind: model.TypeKind(text)}, nil
	}
	if strings.HasPrefix(text, "array<") && strings.HasSuffix(text, ">") {
		inner := strings.TrimSpace(trimmed[6 : len(trimmed)-1])
		if inner == "" {
			return model.TypeSpec{}, fmt.Errorf("array inner type is empty")
		}
		return model.TypeSpec{Raw: strings.ToLower(trimmed[:6]) + inner + ">", Kind: model.TypeArray, Inner: inner}, nil
	}
	if strings.HasPrefix(text, "map<") && strings.HasSuffix(text, ">") {
		args := splitTopLevel(trimmed[4 : len(trimmed)-1])
		if len(args) != 2 {
			return model.TypeSpec{}, fmt.Errorf("map requires key and value types")
		}
		return model.TypeSpec{Raw: "map<" + strings.Join(args, ",") + ">", Kind: model.TypeMap, Args: args}, nil
	}
	if strings.HasPrefix(text, "ref<") && strings.HasSuffix(text, ">") {
		inner := strings.TrimSpace(trimmed[4 : len(trimmed)-1])
		if !identRE.MatchString(inner) {
			return model.TypeSpec{}, fmt.Errorf("ref target %q is not a valid identifier", inner)
		}
		return model.TypeSpec{Raw: "ref<" + inner + ">", Kind: model.TypeRef, Inner: inner}, nil
	}
	return model.TypeSpec{}, fmt.Errorf("unsupported type %q", raw)
}

// ParseUsage 解析字段用途。
// 用途大小写不敏感，支持多个分隔符和别名；client+server 会归一为 all；
// comment 代表纯备注列，不能和任何导出用途组合。
func ParseUsage(raw string) (model.Usage, error) {
	text := strings.ToLower(strings.TrimSpace(raw))
	if text == "" {
		return "", fmt.Errorf("usage is empty")
	}
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '；' || r == '|' || r == '/'
	})
	seenClient := false
	seenServer := false
	seenAll := false
	seenComment := false
	for _, part := range parts {
		token := usageAlias(strings.TrimSpace(part))
		switch token {
		case "client":
			seenClient = true
		case "server":
			seenServer = true
		case "all":
			seenAll = true
		case "comment":
			seenComment = true
		default:
			return "", fmt.Errorf("unsupported usage %q", part)
		}
	}
	if seenComment && (seenClient || seenServer || seenAll) {
		return "", fmt.Errorf("comment usage cannot be combined")
	}
	if seenComment {
		return model.UsageComment, nil
	}
	if seenAll || (seenClient && seenServer) {
		return model.UsageAll, nil
	}
	if seenClient {
		return model.UsageClient, nil
	}
	if seenServer {
		return model.UsageServer, nil
	}
	return "", fmt.Errorf("usage is empty")
}

// ConvertValue 按字段类型把单元格文本转换为 Go 值。
// 返回值中的 bool 表示是否使用默认值：空单元格一定使用默认值，非法值也会回退默认值并返回错误。
// key 字段是否允许为空不在这里处理，而是在 parseSheet 中提前校验。
func ConvertValue(raw string, typ model.TypeSpec) (any, bool, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return defaultValue(typ), true, nil
	}
	switch typ.Kind {
	case model.TypeBool:
		switch strings.ToLower(text) {
		case "true", "1":
			return true, false, nil
		case "false", "0":
			return false, false, nil
		default:
			return defaultValue(typ), true, fmt.Errorf("invalid bool %q", raw)
		}
	case model.TypeInt, model.TypeInt32:
		v, err := strconv.ParseInt(text, 10, 32)
		if err != nil {
			return defaultValue(typ), true, err
		}
		return int32(v), false, nil
	case model.TypeInt64:
		v, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return defaultValue(typ), true, err
		}
		return v, false, nil
	case model.TypeFloat:
		v, err := strconv.ParseFloat(text, 32)
		if err != nil {
			return defaultValue(typ), true, err
		}
		return float32(v), false, nil
	case model.TypeDouble:
		v, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return defaultValue(typ), true, err
		}
		return v, false, nil
	case model.TypeDateTime:
		t, err := time.ParseInLocation("2006-01-02 15:04:05", text, time.Local)
		if err != nil {
			return defaultValue(typ), true, err
		}
		return t.Unix(), false, nil
	case model.TypeArray:
		return strings.Split(text, "|"), false, nil
	case model.TypeMap:
		entries := strings.Split(text, "|")
		out := map[string]string{}
		for _, entry := range entries {
			k, v, ok := strings.Cut(entry, ":")
			if !ok {
				return defaultValue(typ), true, fmt.Errorf("invalid map entry %q", entry)
			}
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
		return out, false, nil
	case model.TypeString, model.TypeBytes, model.TypeRef:
		return text, false, nil
	default:
		return defaultValue(typ), true, fmt.Errorf("unsupported type %s", typ.Raw)
	}
}

// SchemaHash 计算 sheet schema 的稳定摘要。
// 摘要包含 sheet 名、目标、二进制版本、字段名、字段类型、用途、fieldNo、wireType 和是否写入二进制；
// 当前开发阶段不做版本兼容，schemaHash 主要用于非自描述 .bytes 的外部 schema 匹配。
func SchemaHash(sheet model.Sheet, target string) string {
	var b strings.Builder
	b.WriteString(sheet.Name)
	b.WriteString("|")
	b.WriteString(target)
	b.WriteString("|")
	b.WriteString(strconv.FormatUint(constants.BytesFormatVersion, 10))
	b.WriteString("|")
	for _, field := range sheet.Fields {
		b.WriteString(field.Name)
		b.WriteString("|")
		b.WriteString(field.Type.Raw)
		b.WriteString("|")
		b.WriteString(string(field.Usage))
		b.WriteString("|")
		b.WriteString(strconv.FormatUint(field.FieldNo, 10))
		b.WriteString("|")
		b.WriteString(strconv.FormatUint(field.WireType, 10))
		b.WriteString("|")
		b.WriteString(strconv.FormatBool(field.Binary))
		b.WriteString(";")
	}
	hash := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(hash[:])
}

// parseFieldName 解析字段名并识别唯一 key 标记。
// 只允许在字段名前后放一个星号，例如 *id 或 id*；去掉星号后的名称必须满足 identRE。
func parseFieldName(raw string) (string, bool, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", false, fmt.Errorf("field name is empty")
	}
	if strings.Count(name, "*") > 1 {
		return "", false, fmt.Errorf("field name has multiple key markers")
	}
	isKey := strings.HasPrefix(name, "*") || strings.HasSuffix(name, "*")
	if strings.Contains(strings.Trim(name, "*"), "*") {
		return "", false, fmt.Errorf("field name has invalid key marker position")
	}
	name = strings.Trim(name, "*")
	if !identRE.MatchString(name) {
		return "", false, fmt.Errorf("%q is not a valid identifier", raw)
	}
	return name, isKey, nil
}

// defaultValue 返回类型对应的默认值。
// datetime 使用公历元年时间戳，array/map/ref 等引用或复合类型返回 nil。
func defaultValue(typ model.TypeSpec) any {
	switch typ.Kind {
	case model.TypeBool:
		return false
	case model.TypeInt, model.TypeInt32:
		return int32(0)
	case model.TypeInt64:
		return int64(0)
	case model.TypeDateTime:
		return int64(-62135596800)
	case model.TypeFloat:
		return float32(0)
	case model.TypeDouble:
		return float64(0)
	case model.TypeString, model.TypeBytes:
		return ""
	default:
		return nil
	}
}

// usageAlias 把字段用途的简写和历史别名归一成内部枚举文本。
// 这样表格作者可以使用 c/s/srv/cs 等更短写法，同时导出逻辑只处理四种规范用途。
func usageAlias(token string) string {
	switch token {
	case "c", "cli", "clientonly":
		return "client"
	case "s", "srv", "serveronly":
		return "server"
	case "cs", "all", "common", "shared":
		return "all"
	case "note", "remark", "ignore", "skip":
		return "comment"
	default:
		return token
	}
}

// splitTopLevel 拆分 map<K,V> 的两个泛型参数。
// 当前 MVP 类型系统不支持嵌套泛型，因此逗号拆分即可满足需求。
func splitTopLevel(raw string) []string {
	parts := strings.Split(raw, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// isAllowedKeyType 限制唯一 key 字段可使用的类型。
// key 需要稳定、可比较并能作为 C# Dictionary 的索引，因此只允许整数和字符串。
func isAllowedKeyType(kind model.TypeKind) bool {
	return kind == model.TypeInt || kind == model.TypeInt32 || kind == model.TypeInt64 || kind == model.TypeString
}

// wireType 把字段类型映射到 protobuf 风格 wire type。
// signed integer 仍使用 varint wire type，具体符号处理由 binary writer 的 ZigZag 完成。
func wireType(kind model.TypeKind) uint64 {
	switch kind {
	case model.TypeBool, model.TypeInt, model.TypeInt32, model.TypeInt64, model.TypeDateTime:
		return 0
	case model.TypeDouble:
		return 1
	case model.TypeFloat:
		return 5
	default:
		return 2
	}
}

// maxColumns 统计表头区域最大的列数。
// 以表头前 4 行为准可以支持注释行比字段名行更长时及时暴露空字段名错误。
func maxColumns(rows [][]string) int {
	max := 0
	for _, row := range rows {
		if len(row) > max {
			max = len(row)
		}
	}
	return max
}

// cellAt 安全读取单元格文本，并统一 trim 空白。
// 稀疏 Excel 行在 xlsx 读取阶段会补齐，这里仍保留边界保护。
func cellAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// isEmptyRow 判断数据行是否完全为空。
// 全空行不会参与 key 唯一性校验和输出，但会记录到 SkippedEmptyRows 供日志或 JSON 诊断使用。
func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
