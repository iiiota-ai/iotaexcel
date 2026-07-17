package decode

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Options 是 decode 命令写出 csv/json 时使用的配置。
// OutputDir 是输出根目录；Format 只允许 csv/json；Overwrite 控制是否覆盖已有输出文件。
type Options struct {
	OutputDir string
	Format    string
	Overwrite bool
}

// Write 按指定格式写出已解码的 .bytes 文件。
// 输出文件名沿用输入 .bytes 的相对路径，只替换扩展名为 .csv 或 .json。
func Write(file File, opts Options) (string, error) {
	ext := "." + opts.Format
	out := outputPath(opts.OutputDir, file.RelPath, ext)
	if err := ensureWritable(out, opts.Overwrite); err != nil {
		return "", err
	}
	switch opts.Format {
	case "csv":
		return out, writeCSV(out, file)
	case "json":
		return out, writeJSON(out, file)
	default:
		return "", fmt.Errorf("unsupported decode format %q", opts.Format)
	}
}

// outputPath 生成 decode 输出路径。
// 输入是目录时会保留相对路径结构，例如 configs/A.bytes -> out/configs/A.csv。
func outputPath(outputDir, relPath, ext string) string {
	rel := filepath.FromSlash(relPath)
	dir := filepath.Dir(rel)
	base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	if dir == "." {
		return filepath.Join(outputDir, base+ext)
	}
	return filepath.Join(outputDir, dir, base+ext)
}

// ensureWritable 确保输出目录存在，并按 overwrite 参数保护已有文件。
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

// writeCSV 将解码结果写为 CSV。
// 列顺序使用 .bytes 文件头中的字段元数据顺序，缺失值输出为空字符串。
func writeCSV(path string, file File) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	header := make([]string, 0, len(file.Fields))
	for _, field := range file.Fields {
		header = append(header, previewFieldName(field))
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, row := range file.Rows {
		record := make([]string, 0, len(file.Fields))
		for _, field := range file.Fields {
			record = append(record, stringify(row[field.Name]))
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

// writeJSON 将解码结果写为缩进 JSON。
// JSON 会包含 version、schemaHash、字段元数据和行数据，便于排查二进制内容。
func writeJSON(path string, file File) error {
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// stringify 把解码后的值转换为 CSV 单元格文本。
func stringify(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func previewFieldName(field Field) string {
	name := field.Name
	if field.Key {
		return name + "#"
	}
	if field.Unique {
		name += "!"
	}
	if field.Required {
		name += "*"
	}
	return name
}
