package decode

import (
	"fmt"
	"io"

	"iotaexcel/internal/constants"
)

const (
	// PrintVerbose 输出带字段含义说明的人类可读格式。
	PrintVerbose = "verbose"

	// PrintConcise 只输出按读取顺序排列的字面量值和 TLV 数字，不输出说明标签。
	PrintConcise = "concise"
)

// Print 按读取顺序把 .bytes 解码结果输出到终端。
// 输出分为文件说明、头部信息、字段元数据和逐行字段值，适合人工排查二进制内容。
func Print(w io.Writer, file File, mode string) {
	if mode == PrintConcise {
		printConcise(w, file)
		return
	}
	printVerbose(w, file)
}

// printVerbose 输出带说明标签的详细格式。
func printVerbose(w io.Writer, file File) {
	fmt.Fprintf(w, "file: %s\n", file.RelPath)
	fmt.Fprintln(w, "header:")
	fmt.Fprintf(w, "  source: %s\n", file.SourcePath)
	fmt.Fprintf(w, "  version: %d\n", file.Version)
	fmt.Fprintf(w, "  selfDescribing: %t\n", file.SelfDescribing)
	fmt.Fprintf(w, "  schemaHash: %s\n", file.SchemaHash)
	fmt.Fprintf(w, "  keyFieldNo: %d\n", file.KeyFieldNo)
	fmt.Fprintf(w, "  fieldCount: %d\n", len(file.Fields))
	fmt.Fprintf(w, "  rowCount: %d\n", len(file.Rows))

	fmt.Fprintln(w, "fields:")
	for index, field := range file.Fields {
		fmt.Fprintf(w, "  [%d] fieldNo=%d name=%s type=%s flags=%d key=%t required=%t unique=%t\n", index+1, field.FieldNo, field.Name, field.Type.Raw, field.Flags, field.Key, field.Required, field.Unique)
	}

	fmt.Fprintln(w, "rows:")
	for rowIndex, trace := range file.RowTraces {
		fmt.Fprintf(w, "  row %d:\n", rowIndex+1)
		for _, cell := range trace.Cells {
			fmt.Fprintf(w, "    fieldNo=%d name=%s value=%s tag=%d wireType=%d\n", cell.FieldNo, cell.Name, stringify(cell.Value), cell.Tag, cell.WireType)
		}
	}
	fmt.Fprintln(w)
}

// printConcise 输出不带说明文本的简洁格式。
// 行格式依次为：文件路径、源路径、magic、版本、自描述标记、schemaHash、keyFieldNo、字段数、
// 每个字段的 fieldNo/name/type、行数、每个单元格的 tag/fieldNo/wireType/value。
func printConcise(w io.Writer, file File) {
	fmt.Fprintln(w, file.RelPath)
	fmt.Fprintln(w, file.SourcePath)
	fmt.Fprintln(w, constants.BytesMagic)
	fmt.Fprintln(w, file.Version)
	fmt.Fprintln(w, file.SelfDescribing)
	fmt.Fprintln(w, file.SchemaHash)
	fmt.Fprintln(w, file.KeyFieldNo)
	fmt.Fprintln(w, len(file.Fields))
	for _, field := range file.Fields {
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\n", field.FieldNo, field.Name, field.Type.Raw, field.Flags)
	}
	fmt.Fprintln(w, len(file.RowTraces))
	for _, trace := range file.RowTraces {
		for _, cell := range trace.Cells {
			fmt.Fprintf(w, "%d\t%d\t%d\t%s\n", cell.Tag, cell.FieldNo, cell.WireType, stringify(cell.Value))
		}
	}
	fmt.Fprintln(w)
}
