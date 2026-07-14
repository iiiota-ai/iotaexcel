// Package xlsx 提供最小 XLSX 读取能力。
//
// 该包直接读取 .xlsx(zip) 内的 XML 文件，只解析配置导出所需的 workbook、relationship、
// worksheet、sharedStrings 和 inlineStr，不依赖第三方 Excel 库，以保持工具可静态编译和包体较小。
package xlsx

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"

	"iotaexcel/internal/model"
)

// Read 读取一个 .xlsx 文件并返回原始字符串矩阵。
// sheetFilter 为空时读取所有 sheet；非空时支持按 sheet 名或从 1 开始的 sheet 索引筛选。
// relPath 会原样带到 RawWorkbook 中，供后续输出阶段保留相对目录结构。
func Read(filePath, relPath, sheetFilter string) (model.RawWorkbook, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return model.RawWorkbook{}, err
	}
	defer zr.Close()

	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[strings.ReplaceAll(f.Name, "\\", "/")] = f
	}

	shared, err := readSharedStrings(files["xl/sharedStrings.xml"])
	if err != nil {
		return model.RawWorkbook{}, err
	}
	sheets, err := readWorkbookSheets(files)
	if err != nil {
		return model.RawWorkbook{}, err
	}

	var out []model.RawSheet
	for i, sheet := range sheets {
		if sheetFilter != "" && !matchesSheet(sheetFilter, sheet.Name, i+1) {
			continue
		}
		rows, err := readWorksheet(files[sheet.Path], shared)
		if err != nil {
			return model.RawWorkbook{}, fmt.Errorf("sheet %s: %w", sheet.Name, err)
		}
		out = append(out, model.RawSheet{Name: sheet.Name, Rows: rows})
	}
	if sheetFilter != "" && len(out) == 0 {
		return model.RawWorkbook{}, fmt.Errorf("sheet %q not found", sheetFilter)
	}
	return model.RawWorkbook{SourcePath: filePath, RelPath: relPath, Sheets: out}, nil
}

// workbookSheet 是 workbook.xml 和 workbook.xml.rels 合并后的 sheet 元数据。
// Name 是用户看到的 sheet 名，RID 用于关联 relationship，Path 是 worksheet XML 在 zip 中的位置。
type workbookSheet struct {
	Name string
	RID  string
	Path string
}

// readWorkbookSheets 读取工作簿中的 sheet 列表和对应 worksheet 路径。
// XLSX 中 sheet 名和实际 XML 文件路径分散在 workbook.xml 与 relationship 文件中，需要合并解析。
func readWorkbookSheets(files map[string]*zip.File) ([]workbookSheet, error) {
	wb := files["xl/workbook.xml"]
	rels := files["xl/_rels/workbook.xml.rels"]
	if wb == nil || rels == nil {
		return nil, fmt.Errorf("missing workbook metadata")
	}

	type sheetXML struct {
		Name string `xml:"name,attr"`
		RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	}
	type workbookXML struct {
		Sheets []sheetXML `xml:"sheets>sheet"`
	}
	var workbook workbookXML
	if err := decodeXML(wb, &workbook); err != nil {
		return nil, err
	}

	type relXML struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	}
	type relsXML struct {
		Relations []relXML `xml:"Relationship"`
	}
	var relationships relsXML
	if err := decodeXML(rels, &relationships); err != nil {
		return nil, err
	}
	targets := map[string]string{}
	for _, rel := range relationships.Relations {
		target := strings.TrimPrefix(rel.Target, "/")
		if !strings.HasPrefix(target, "xl/") {
			target = path.Clean("xl/" + target)
		}
		targets[rel.ID] = target
	}

	var sheets []workbookSheet
	for _, sheet := range workbook.Sheets {
		target := targets[sheet.RID]
		if target == "" {
			return nil, fmt.Errorf("missing relationship for sheet %s", sheet.Name)
		}
		sheets = append(sheets, workbookSheet{Name: sheet.Name, RID: sheet.RID, Path: target})
	}
	return sheets, nil
}

// readSharedStrings 读取共享字符串表。
// 许多 Excel 会把重复字符串集中放在 xl/sharedStrings.xml，单元格中只保存索引。
func readSharedStrings(file *zip.File) ([]string, error) {
	if file == nil {
		return nil, nil
	}
	type item struct {
		Texts []string `xml:"t"`
	}
	type sst struct {
		Items []item `xml:"si"`
	}
	var data sst
	if err := decodeXML(file, &data); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(data.Items))
	for _, it := range data.Items {
		out = append(out, strings.Join(it.Texts, ""))
	}
	return out, nil
}

// readWorksheet 读取单个 worksheet XML 并还原成二维字符串矩阵。
// 这里会按单元格引用（如 C5）补齐稀疏行列，确保 schema 层可以通过列下标稳定读取表头和数据。
func readWorksheet(file *zip.File, shared []string) ([][]string, error) {
	if file == nil {
		return nil, fmt.Errorf("missing worksheet xml")
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	type row struct {
		Index int             `xml:"r,attr"`
		Cells []worksheetCell `xml:"c"`
	}
	type worksheet struct {
		Rows []row `xml:"sheetData>row"`
	}
	var ws worksheet
	if err := xml.NewDecoder(rc).Decode(&ws); err != nil {
		return nil, err
	}

	maxCol := 0
	rowValues := map[int]map[int]string{}
	for _, r := range ws.Rows {
		if r.Index == 0 {
			r.Index = len(rowValues) + 1
		}
		if rowValues[r.Index] == nil {
			rowValues[r.Index] = map[int]string{}
		}
		for _, c := range r.Cells {
			col := colIndex(c.Ref)
			if col == 0 {
				col = len(rowValues[r.Index]) + 1
			}
			if col > maxCol {
				maxCol = col
			}
			rowValues[r.Index][col] = cellText(c, shared)
		}
	}

	maxRow := 0
	for idx := range rowValues {
		if idx > maxRow {
			maxRow = idx
		}
	}
	rows := make([][]string, maxRow)
	for r := 1; r <= maxRow; r++ {
		rows[r-1] = make([]string, maxCol)
		for c := 1; c <= maxCol; c++ {
			rows[r-1][c-1] = rowValues[r][c]
		}
	}
	return rows, nil
}

// worksheetCell 映射 worksheet 中的 c 节点。
// Type 为 s 时 Value 是 sharedStrings 索引；Type 为 inlineStr 时文本位于 InlineStr.Text。
type worksheetCell struct {
	Ref       string `xml:"r,attr"`
	Type      string `xml:"t,attr"`
	Value     string `xml:"v"`
	InlineStr struct {
		Text string `xml:"t"`
	} `xml:"is"`
}

// cellText 按单元格类型解析最终文本。
// 不支持公式求值、样式格式化等 Excel 高级特性，读取到的都是 XML 中保存的原始文本。
func cellText(c worksheetCell, shared []string) string {
	switch c.Type {
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(c.Value))
		if err == nil && idx >= 0 && idx < len(shared) {
			return shared[idx]
		}
		return ""
	case "inlineStr":
		return c.InlineStr.Text
	default:
		return c.Value
	}
}

// colRE 用于从 A1、BC23 这类单元格引用中提取列字母。
var colRE = regexp.MustCompile(`^[A-Z]+`)

// colIndex 把 Excel 列名转换成从 1 开始的列号，例如 A=1、Z=26、AA=27。
func colIndex(ref string) int {
	letters := colRE.FindString(strings.ToUpper(ref))
	n := 0
	for _, r := range letters {
		n = n*26 + int(r-'A'+1)
	}
	return n
}

// matchesSheet 判断 --sheet 参数是否命中当前 sheet。
// 为了方便命令行使用，既允许传 sheet 名，也允许传从 1 开始的 sheet 序号。
func matchesSheet(filter, name string, index int) bool {
	if filter == name {
		return true
	}
	i, err := strconv.Atoi(filter)
	return err == nil && i == index
}

// decodeXML 读取 zip 中的 XML 文件并反序列化到目标结构。
// 这里先完整读入内存，配置表 XML 通常较小，换取实现简单和错误信息清晰。
func decodeXML(file *zip.File, v any) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	return xml.Unmarshal(data, v)
}
