// Package decode 提供 .bytes 文件的反解析能力。
//
// decode 命令使用该包读取 convert 写出的 protobuf 风格 TLV payload，
// 再根据文件内嵌的字段元数据输出为 CSV 或 JSON，便于人工排查或和 Excel 原始数据对照。
package decode

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"iotaexcel/internal/constants"
	"iotaexcel/internal/model"
	"iotaexcel/internal/schema"
)

// Field 是 .bytes 文件内嵌字段元数据的结构化表示。
// FieldNo 来自 Excel 列号，Name/Type 用于还原输出列名和按类型解码字段值。
type Field struct {
	FieldNo  uint64         `json:"fieldNo"`
	Name     string         `json:"name"`
	Type     model.TypeSpec `json:"type"`
	Flags    uint64         `json:"flags"`
	Key      bool           `json:"key"`
	Required bool           `json:"required"`
	Unique   bool           `json:"unique"`
}

// File 是一个 .bytes 文件解码后的完整内容。
// Rows 中每一行按字段名保存已还原的值，JSON 输出会直接序列化该结构。
type File struct {
	SourcePath     string           `json:"source"`
	RelPath        string           `json:"relPath"`
	Version        uint64           `json:"version"`
	SelfDescribing bool             `json:"selfDescribing"`
	SchemaHash     string           `json:"schemaHash"`
	KeyFieldNo     uint64           `json:"keyFieldNo"`
	Fields         []Field          `json:"fields"`
	Rows           []map[string]any `json:"rows"`
	RowTraces      []RowTrace       `json:"-"`
}

// ReadOptions 控制 .bytes 解码行为。
// Schemas 用 schemaHash 关联外部 Excel schema，供非自描述 .bytes 恢复字段名和类型。
type ReadOptions struct {
	Schemas map[string][]Field
}

// RowTrace 保存一行数据的 TLV 读取轨迹。
// 它不进入 JSON 输出，只用于 --print 按读取顺序展示 tag、fieldNo、wireType 和字面量值。
type RowTrace struct {
	Cells []CellTrace
}

// CellTrace 保存单个单元格的 TLV 信息和解码后的字面量值。
type CellTrace struct {
	Tag      uint64
	FieldNo  uint64
	WireType uint64
	Name     string
	Value    any
}

// Read 读取并解码单个 .bytes 文件。
// relPath 会保存在结果中，用于批量处理时追踪输入来源。
func Read(path, relPath string) (File, error) {
	return ReadWithOptions(path, relPath, ReadOptions{})
}

// ReadWithOptions 读取并解码单个 .bytes 文件，并允许传入外部 schema。
func ReadWithOptions(path, relPath string, opts ReadOptions) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	reader := byteReader{data: data}
	if got := string(reader.readN(len(constants.BytesMagic))); got != constants.BytesMagic {
		return File{}, fmt.Errorf("invalid magic %q", got)
	}
	if reader.err != nil {
		return File{}, reader.err
	}

	out := File{SourcePath: path, RelPath: relPath}
	out.Version = reader.uvarint()
	if out.Version != constants.BytesFormatVersion {
		return File{}, fmt.Errorf("unsupported bytes version %d, want %d", out.Version, constants.BytesFormatVersion)
	}
	out.SchemaHash = string(reader.bytes())
	out.KeyFieldNo = reader.uvarint()
	out.SelfDescribing = reader.uvarint() != 0

	fieldCount := reader.uvarint()
	fieldsByNo := map[uint64]Field{}
	if out.SelfDescribing {
		for i := uint64(0); i < fieldCount; i++ {
			fieldNo := reader.uvarint()
			name := string(reader.bytes())
			typeRaw := string(reader.bytes())
			typeSpec, err := schema.ParseType(typeRaw)
			if err != nil {
				return File{}, fmt.Errorf("field %s type %q: %w", name, typeRaw, err)
			}
			flags := reader.uvarint()
			field := fieldFromFlags(fieldNo, name, typeSpec, flags)
			out.Fields = append(out.Fields, field)
			fieldsByNo[fieldNo] = field
		}
	} else {
		schemaFields := opts.Schemas[out.SchemaHash]
		if len(schemaFields) == 0 {
			return File{}, fmt.Errorf("non-self-describing bytes require --schema-input matching schemaHash %s", out.SchemaHash)
		}
		schemaByNo := map[uint64]Field{}
		for _, field := range schemaFields {
			schemaByNo[field.FieldNo] = field
		}
		for i := uint64(0); i < fieldCount; i++ {
			fieldNo := reader.uvarint()
			field, ok := schemaByNo[fieldNo]
			if !ok {
				return File{}, fmt.Errorf("schemaHash %s missing fieldNo %d", out.SchemaHash, fieldNo)
			}
			out.Fields = append(out.Fields, field)
			fieldsByNo[fieldNo] = field
		}
	}

	rowCount := reader.uvarint()
	out.Rows = make([]map[string]any, 0, rowCount)
	out.RowTraces = make([]RowTrace, 0, rowCount)
	for i := uint64(0); i < rowCount; i++ {
		row, trace, err := decodeRow(reader.bytes(), fieldsByNo)
		if err != nil {
			return File{}, fmt.Errorf("row %d: %w", i+1, err)
		}
		out.Rows = append(out.Rows, row)
		out.RowTraces = append(out.RowTraces, trace)
	}
	if reader.err != nil {
		return File{}, reader.err
	}
	if !reader.end() {
		return File{}, fmt.Errorf("trailing bytes: %d", len(reader.data)-reader.pos)
	}
	return out, nil
}

func fieldFromFlags(fieldNo uint64, name string, typ model.TypeSpec, flags uint64) Field {
	return Field{
		FieldNo:  fieldNo,
		Name:     name,
		Type:     typ,
		Flags:    flags,
		Key:      flags&constants.FieldFlagKey != 0,
		Required: flags&constants.FieldFlagRequired != 0,
		Unique:   flags&constants.FieldFlagUnique != 0,
	}
}

// decodeRow 解码一行 TLV payload。
// 未知 fieldNo 会按 wireType 跳过，避免单个无法识别字段阻断整行已知字段的读取。
func decodeRow(data []byte, fields map[uint64]Field) (map[string]any, RowTrace, error) {
	reader := byteReader{data: data}
	row := map[string]any{}
	trace := RowTrace{}
	for !reader.end() {
		tag := reader.uvarint()
		fieldNo := tag >> 3
		wireType := tag & 7
		field, ok := fields[fieldNo]
		if !ok {
			if err := reader.skip(wireType); err != nil {
				return nil, trace, err
			}
			continue
		}
		value, err := reader.value(field.Type, wireType)
		if err != nil {
			return nil, trace, fmt.Errorf("field %s: %w", field.Name, err)
		}
		row[field.Name] = value
		trace.Cells = append(trace.Cells, CellTrace{Tag: tag, FieldNo: fieldNo, WireType: wireType, Name: field.Name, Value: value})
	}
	return row, trace, nil
}

// byteReader 是 .bytes 解码专用的顺序读取器。
// 所有读取方法都会推进 pos，并在格式非法或越界时记录错误。
type byteReader struct {
	data []byte
	pos  int
	err  error
}

// end 判断读取器是否到达末尾。
func (r *byteReader) end() bool {
	return r.pos >= len(r.data)
}

// uvarint 读取无符号 varint。
func (r *byteReader) uvarint() uint64 {
	if r.err != nil {
		return 0
	}
	value, n := binary.Uvarint(r.data[r.pos:])
	if n <= 0 {
		r.err = fmt.Errorf("invalid varint at %d", r.pos)
		return 0
	}
	r.pos += n
	return value
}

// bytes 读取 length-delimited 字段。
func (r *byteReader) bytes() []byte {
	size := r.uvarint()
	return r.readN(int(size))
}

// readN 读取固定数量字节。
func (r *byteReader) readN(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || r.pos+n > len(r.data) {
		r.err = fmt.Errorf("read beyond end: pos=%d n=%d len=%d", r.pos, n, len(r.data))
		return nil
	}
	out := r.data[r.pos : r.pos+n]
	r.pos += n
	return out
}

// value 根据字段类型和 wireType 读取一个字段值。
// wireType 来自行内 tag，TypeSpec 来自文件头中的字段元数据，二者共同决定解码方式。
func (r *byteReader) value(typ model.TypeSpec, wireType uint64) (any, error) {
	switch wireType {
	case 0:
		raw := r.uvarint()
		if r.err != nil {
			return nil, r.err
		}
		if typ.Kind == model.TypeBool {
			return raw != 0, nil
		}
		return unzigzag(raw), nil
	case 1:
		raw := r.readN(8)
		if r.err != nil {
			return nil, r.err
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(raw)), nil
	case 2:
		text := string(r.bytes())
		if r.err != nil {
			return nil, r.err
		}
		return text, nil
	case 5:
		raw := r.readN(4)
		if r.err != nil {
			return nil, r.err
		}
		return math.Float32frombits(binary.LittleEndian.Uint32(raw)), nil
	default:
		return nil, fmt.Errorf("unsupported wire type %d", wireType)
	}
}

// skip 跳过未知字段。
// 这样在字段元数据缺失时，已知字段仍有机会被解码输出。
func (r *byteReader) skip(wireType uint64) error {
	switch wireType {
	case 0:
		r.uvarint()
	case 1:
		r.readN(8)
	case 2:
		r.bytes()
	case 5:
		r.readN(4)
	default:
		return fmt.Errorf("unsupported wire type %d", wireType)
	}
	return r.err
}

// unzigzag 将 ZigZag 编码的无符号整数还原为有符号整数。
func unzigzag(value uint64) int64 {
	return int64(value>>1) ^ -int64(value&1)
}
