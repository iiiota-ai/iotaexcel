// Package convert 提供把结构化 Workbook 写成不同输出格式的能力。
//
// binary.go 实现 .bytes 输出，编码方式借鉴 protobuf wire format：
// 字段使用 tag=(fieldNo<<3)|wireType，整数使用 varint，带符号整数再叠加 ZigZag。
package convert

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"iotaexcel/internal/constants"
	"iotaexcel/internal/model"
)

// WriteBinary 将 workbook 中每个 sheet 分别写成一个 .bytes 文件。
// 输出文件名由 Excel 文件名和 sheet 名组成，目录结构由 outputBase 统一处理。
func WriteBinary(wb model.Workbook, opts Options) ([]string, error) {
	var outputs []string
	for _, sheet := range wb.Sheets {
		out := outputBase(opts.OutputDir, wb, sheet, constants.BytesExtension)
		if err := ensureWritable(out, opts.Overwrite); err != nil {
			return outputs, err
		}
		data, err := encodeSheet(sheet, opts.Target, !opts.OmitSelfDescription)
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

// encodeSheet 把单个 sheet 编码为完整 .bytes payload。
// 外层结构依次写入 magic、版本号、schemaHash、key fieldNo、自描述标记、字段元数据和逐行 TLV 数据。
func encodeSheet(sheet model.Sheet, target string, selfDescribing bool) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(constants.BytesMagic)
	writeUvarint(&buf, constants.BytesFormatVersion)
	writeBytes(&buf, []byte(sheet.SchemaHash))

	fields := BinaryFields(sheet.Fields, target)
	keyFieldNo := uint64(0)
	for _, field := range fields {
		if field.IsKey {
			keyFieldNo = field.FieldNo
			break
		}
	}
	if keyFieldNo == 0 {
		return nil, fmt.Errorf("key field is not included for target %s", target)
	}
	writeUvarint(&buf, keyFieldNo)
	if selfDescribing {
		writeUvarint(&buf, 1)
	} else {
		writeUvarint(&buf, 0)
	}
	writeUvarint(&buf, uint64(len(fields)))
	for _, field := range fields {
		writeUvarint(&buf, field.FieldNo)
		if selfDescribing {
			writeString(&buf, field.Name)
			writeString(&buf, field.Type.Raw)
		}
	}
	writeUvarint(&buf, uint64(len(sheet.Rows)))
	for _, row := range sheet.Rows {
		var rowBuf bytes.Buffer
		for _, field := range fields {
			cell := row.Values[field.Name]
			writeTag(&rowBuf, field.FieldNo, field.WireType)
			if err := writeValue(&rowBuf, field.Type, cell.Value); err != nil {
				return nil, fmt.Errorf("row %d field %s: %w", row.Index, field.Name, err)
			}
		}
		writeBytes(&buf, rowBuf.Bytes())
	}
	return buf.Bytes(), nil
}

// writeValue 按字段类型写入单个单元格值。
// wireType 已经在 schema 阶段确定，这里只负责把 Go 值转换成对应的二进制字节序列。
func writeValue(buf *bytes.Buffer, typ model.TypeSpec, value any) error {
	switch typ.Kind {
	case model.TypeBool:
		if asBool(value) {
			writeUvarint(buf, 1)
		} else {
			writeUvarint(buf, 0)
		}
	case model.TypeInt, model.TypeInt32:
		writeUvarint(buf, zigzag(int64(asInt32(value))))
	case model.TypeInt64, model.TypeDateTime:
		writeUvarint(buf, zigzag(asInt64(value)))
	case model.TypeFloat:
		var tmp [4]byte
		binary.LittleEndian.PutUint32(tmp[:], math.Float32bits(asFloat32(value)))
		buf.Write(tmp[:])
	case model.TypeDouble:
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(asFloat64(value)))
		buf.Write(tmp[:])
	default:
		writeString(buf, stringValue(value, typ))
	}
	return nil
}

// stringValue 把 length-delimited 类型转换为稳定文本。
// array 沿用 Excel 中的 | 分隔形式；map 会按 key 排序，避免 Go map 遍历顺序影响二进制输出。
func stringValue(value any, typ model.TypeSpec) string {
	if value == nil {
		return ""
	}
	switch typ.Kind {
	case model.TypeArray:
		items, ok := value.([]string)
		if !ok {
			return ""
		}
		return strings.Join(items, "|")
	case model.TypeMap:
		items, ok := value.(map[string]string)
		if !ok {
			return ""
		}
		keys := make([]string, 0, len(items))
		for key := range items {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, key+":"+items[key])
		}
		return strings.Join(parts, "|")
	default:
		return fmt.Sprint(value)
	}
}

// writeTag 写入 protobuf 风格 tag。
// 高位保存字段号，低 3 位保存 wire type。
func writeTag(buf *bytes.Buffer, fieldNo, wireType uint64) {
	writeUvarint(buf, fieldNo<<3|wireType)
}

// writeBytes 写入 length-delimited 数据。
// 先写 varint 长度，再写原始字节内容。
func writeBytes(buf *bytes.Buffer, data []byte) {
	writeUvarint(buf, uint64(len(data)))
	buf.Write(data)
}

// writeString 以 UTF-8 字节写入字符串。
// Go 字符串本身按字节保存，这里直接转换为 []byte。
func writeString(buf *bytes.Buffer, value string) {
	writeBytes(buf, []byte(value))
}

// writeUvarint 写入无符号 varint。
// 使用标准库 binary.PutUvarint，保证和 Go 侧测试解码器一致。
func writeUvarint(buf *bytes.Buffer, value uint64) {
	var tmp [10]byte
	n := binary.PutUvarint(tmp[:], value)
	buf.Write(tmp[:n])
}

// zigzag 将有符号整数转换成适合 varint 的无符号整数。
// 这样 -1、-2 等小负数也能编码成较短字节序列。
func zigzag(value int64) uint64 {
	return uint64(value<<1) ^ uint64(value>>63)
}

// asBool 从 any 中取 bool，类型不匹配时返回默认 false。
// schema 转换阶段已经保证正常值类型正确，这里主要作为防御性边界。
func asBool(value any) bool {
	v, _ := value.(bool)
	return v
}

// asInt32 把整数类值收敛到 int32。
// int 类型只用于测试或调用方手工构造数据，正常 schema 输出为 int32。
func asInt32(value any) int32 {
	switch v := value.(type) {
	case int32:
		return v
	case int:
		return int32(v)
	case int64:
		return int32(v)
	default:
		return 0
	}
}

// asInt64 把整数类值收敛到 int64。
// int32/int 也允许进入该路径，便于默认值或测试数据复用。
func asInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	default:
		return 0
	}
}

// asFloat32 把浮点值收敛到 float32。
// schema 解析 float 时已经使用 32 位精度，这里保留 float64 转换以兼容手工构造数据。
func asFloat32(value any) float32 {
	switch v := value.(type) {
	case float32:
		return v
	case float64:
		return float32(v)
	default:
		return 0
	}
}

// asFloat64 把浮点值收敛到 float64。
// float32 也可以提升为 float64 写入 double 字段。
func asFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	default:
		return 0
	}
}
