// Package convert 的 csv.go 提供 CSV 调试输出。
//
// CSV 不是最终运行时格式，主要用于人工检查 schema 解析后的行列是否符合预期。
package convert

import (
	"encoding/csv"
	"os"

	"iotaexcel/internal/model"
)

// WriteCSV 将 workbook 中每个 sheet 输出为 CSV 文件。
// 当前 CSV 使用原始单元格文本，comment 字段会被跳过，便于和 Excel 中看到的内容直接对照。
func WriteCSV(wb model.Workbook, opts Options) ([]string, error) {
	var outputs []string
	for _, sheet := range wb.Sheets {
		out := outputBase(opts.OutputDir, wb, sheet, ".csv")
		if err := ensureWritable(out, opts.Overwrite); err != nil {
			return outputs, err
		}
		f, err := os.Create(out)
		if err != nil {
			return outputs, err
		}
		w := csv.NewWriter(f)

		header := make([]string, 0, len(sheet.Fields))
		for _, field := range sheet.Fields {
			if field.Usage != model.UsageComment {
				header = append(header, field.Name)
			}
		}
		if err := w.Write(header); err != nil {
			f.Close()
			return outputs, err
		}
		for _, row := range sheet.Rows {
			record := make([]string, 0, len(header))
			for _, field := range sheet.Fields {
				if field.Usage != model.UsageComment {
					record = append(record, row.Values[field.Name].Raw)
				}
			}
			if err := w.Write(record); err != nil {
				f.Close()
				return outputs, err
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			f.Close()
			return outputs, err
		}
		if err := f.Close(); err != nil {
			return outputs, err
		}
		outputs = append(outputs, out)
	}
	return outputs, nil
}
