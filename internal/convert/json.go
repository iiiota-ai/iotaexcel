// Package convert 的 json.go 提供结构化 JSON 输出。
//
// JSON 输出主要用于调试和测试：它保留 schemaHash、字段元数据、转换错误、
// 默认值统计等信息，便于观察 Excel 解析后的完整内部模型。
package convert

import (
	"encoding/json"
	"os"

	"iotaexcel/internal/model"
)

// WriteJSON 将 workbook 中每个 sheet 输出为一个缩进格式的 JSON 文件。
// payload 中同时包含源文件相对路径和解析后的 sheet，方便批量导出时定位数据来源。
func WriteJSON(wb model.Workbook, opts Options) ([]string, error) {
	var outputs []string
	for _, sheet := range wb.Sheets {
		out := outputBase(opts.OutputDir, wb, sheet, ".json")
		if err := ensureWritable(out, opts.Overwrite); err != nil {
			return outputs, err
		}
		payload := struct {
			Source string      `json:"source"`
			Sheet  model.Sheet `json:"sheet"`
		}{Source: wb.RelPath, Sheet: sheet}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return outputs, err
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return outputs, err
		}
		outputs = append(outputs, out)
	}
	return outputs, nil
}
