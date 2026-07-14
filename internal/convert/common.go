// Package convert 包含各类导出格式的共享选项和辅助函数。
//
// 具体格式分别由 csv.go、json.go、binary.go 实现；common.go 放置输出路径、
// 覆盖保护和字段用途过滤等公共逻辑。
package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"iotaexcel/internal/constants"
	"iotaexcel/internal/model"
)

// Options 是所有导出格式共用的配置。
// OutputDir 是输出根目录；Target 控制 client/server/all 字段过滤；Overwrite 控制是否允许覆盖已有文件。
// OmitSelfDescription 仅对 .bytes 生效：开启时不内嵌字段名和类型名，decode 需要外部 schema。
type Options struct {
	OutputDir           string
	Target              string
	Overwrite           bool
	OmitSelfDescription bool
}

// outputBase 生成单个 sheet 的输出路径。
// 文件名格式为“Excel文件名_sheet名.扩展名”，并在输入是目录时保留 workbook 的相对目录结构。
func outputBase(outputDir string, wb model.Workbook, sheet model.Sheet, ext string) string {
	rel := filepath.FromSlash(wb.RelPath)
	dir := filepath.Dir(rel)
	base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	sheetName := sheet.Name
	if ext == constants.BytesExtension {
		sheetName += constants.ConfigSuffix
	}
	name := fmt.Sprintf("%s_%s%s", base, sheetName, ext)
	if dir == "." {
		return filepath.Join(outputDir, name)
	}
	return filepath.Join(outputDir, dir, name)
}

// ensureWritable 确保输出目录存在，并在未开启 overwrite 时阻止覆盖已有文件。
// 所有格式写文件前都先调用该函数，保证覆盖语义一致。
func ensureWritable(path string, overwrite bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("output exists: %s", path)
		}
	}
	return nil
}

// includeField 判断字段是否应该进入当前导出目标。
// Binary=false 或 UsageComment 的字段会被排除；client/server 目标只保留对应字段和 all 字段。
func includeField(field model.Field, target string) bool {
	if !field.Binary {
		return false
	}
	switch target {
	case "client":
		return field.Usage == model.UsageClient || field.Usage == model.UsageAll
	case "server":
		return field.Usage == model.UsageServer || field.Usage == model.UsageAll
	default:
		return field.Usage != model.UsageComment
	}
}

// BinaryFields 根据导出目标返回实际进入 .bytes 的字段集合。
// decode 使用外部 schema 解析非自描述 .bytes 时也复用这套过滤规则，保证 fieldNo 顺序一致。
func BinaryFields(fields []model.Field, target string) []model.Field {
	out := make([]model.Field, 0, len(fields))
	for _, field := range fields {
		if includeField(field, target) {
			out = append(out, field)
		}
	}
	return out
}
