package decode

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"iotaexcel/internal/constants"
	"iotaexcel/internal/model"
)

func TestDecodeRowSkipsUnknownFields(t *testing.T) {
	fields := map[uint64]Field{
		1: {FieldNo: 1, Name: "id", Type: model.TypeSpec{Raw: "int", Kind: model.TypeInt}},
		2: {FieldNo: 2, Name: "name", Type: model.TypeSpec{Raw: "string", Kind: model.TypeString}},
	}

	var row []byte
	row = appendUvarint(row, 9<<3|0)
	row = appendUvarint(row, 123)
	row = appendUvarint(row, 1<<3|0)
	row = appendUvarint(row, 5)
	row = appendUvarint(row, 2<<3|2)
	row = appendBytes(row, []byte("Sword"))

	decoded, trace, err := decodeRow(row, fields)
	if err != nil {
		t.Fatal(err)
	}
	if decoded["id"] != int64(-3) || decoded["name"] != "Sword" {
		t.Fatalf("decoded row = %#v", decoded)
	}
	if len(trace.Cells) != 2 {
		t.Fatalf("trace cells = %d, want 2", len(trace.Cells))
	}
}

func TestReadWithOptionsRequiresSchemaForSlimBytes(t *testing.T) {
	data := []byte(constants.BytesMagic)
	data = appendUvarint(data, constants.BytesFormatVersion)
	data = appendBytes(data, []byte("missing-schema"))
	data = appendUvarint(data, 1)
	data = appendUvarint(data, 0)
	data = appendUvarint(data, 1)
	data = appendUvarint(data, 1)
	data = appendUvarint(data, 0)

	path := filepath.Join(t.TempDir(), "Config_ItemConfig.bytes")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadWithOptions(path, "Config_ItemConfig.bytes", ReadOptions{})
	if err == nil || !strings.Contains(err.Error(), "require --schema-input") {
		t.Fatalf("err = %v, want schema-input error", err)
	}
}

func TestReadSelfDescribingFieldFlags(t *testing.T) {
	data := []byte(constants.BytesMagic)
	data = appendUvarint(data, constants.BytesFormatVersion)
	data = appendBytes(data, []byte("schema"))
	data = appendUvarint(data, 1)
	data = appendUvarint(data, 1)
	data = appendUvarint(data, 1)
	data = appendUvarint(data, 1)
	data = appendBytes(data, []byte("id"))
	data = appendBytes(data, []byte("int"))
	data = appendUvarint(data, constants.FieldFlagKey|constants.FieldFlagRequired|constants.FieldFlagUnique)
	data = appendUvarint(data, 0)

	path := filepath.Join(t.TempDir(), "Config_ItemConfig.bytes")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	decoded, err := Read(path, "Config_ItemConfig.bytes")
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Fields) != 1 {
		t.Fatalf("fields = %d, want 1", len(decoded.Fields))
	}
	field := decoded.Fields[0]
	if field.Flags != constants.FieldFlagKey|constants.FieldFlagRequired|constants.FieldFlagUnique || !field.Key || !field.Required || !field.Unique {
		t.Fatalf("field flags = %#v", field)
	}
}

func TestReadRejectsInvalidMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.bytes")
	if err := os.WriteFile(path, []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Read(path, "bad.bytes")
	if err == nil || !strings.Contains(err.Error(), "invalid magic") {
		t.Fatalf("err = %v, want invalid magic", err)
	}
}

func appendUvarint(dst []byte, value uint64) []byte {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], value)
	return append(dst, buf[:n]...)
}

func appendBytes(dst, value []byte) []byte {
	dst = appendUvarint(dst, uint64(len(value)))
	return append(dst, value...)
}
