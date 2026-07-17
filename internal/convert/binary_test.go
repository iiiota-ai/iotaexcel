package convert

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"iotaexcel/internal/constants"
	"iotaexcel/internal/schema"
	"iotaexcel/internal/xlsx"
)

// TestWriteBinaryEncodesTLVRows 验证 .bytes 写入格式是否能按预期解码。
// 该测试不只检查文件存在，还会把输出字节重新解析为 TLV，确认 magic、版本号、
// key fieldNo、comment 字段过滤、ZigZag、定长浮点和 array/map 文本载荷都稳定。
func TestWriteBinaryEncodesTLVRows(t *testing.T) {
	root := repoRoot(t)
	raw, err := xlsx.Read(
		filepath.Join(root, "tests", "testdata", "excels", "valid", "Config.xlsx"),
		"Config.xlsx",
		"Item",
	)
	if err != nil {
		t.Fatal(err)
	}
	wb, err := schema.ParseWorkbook(raw, schema.Options{Target: "both"})
	if err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	outputs, err := WriteBinary(wb, Options{OutputDir: out, Target: "both", Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 1 {
		t.Fatalf("outputs = %d, want 1", len(outputs))
	}

	data, err := os.ReadFile(outputs[0])
	if err != nil {
		t.Fatal(err)
	}
	decoded := decodeBytesFixture(t, data)

	if decoded.version != constants.BytesFormatVersion {
		t.Fatalf("version = %d, want %d", decoded.version, constants.BytesFormatVersion)
	}
	if !decoded.selfDescribing {
		t.Fatalf("selfDescribing = false, want true")
	}
	if decoded.keyFieldNo != 1 {
		t.Fatalf("keyFieldNo = %d, want 1", decoded.keyFieldNo)
	}
	if decoded.rowCount != 2 {
		t.Fatalf("rowCount = %d, want 2", decoded.rowCount)
	}
	if _, ok := decoded.fields[12]; ok {
		t.Fatalf("comment fieldNo 12 should not be encoded")
	}

	first := decoded.rows[0]
	if got := first.varints[1]; got != 2002 { // ZigZag(1001)
		t.Fatalf("id raw varint = %d, want 2002", got)
	}
	if got := unzigzag(first.varints[5]); got != -12 {
		t.Fatalf("score = %d, want -12", got)
	}
	if got := unzigzag(first.varints[6]); got != -900719925474 {
		t.Fatalf("big = %d, want -900719925474", got)
	}
	if got := first.strings[2]; got != "Sword" {
		t.Fatalf("name = %q, want Sword", got)
	}
	if got, want := unzigzag(first.varints[9]), fixtureUnix(t, "2026-07-10 18:47:00"); got != want {
		t.Fatalf("createdAt = %d, want %d", got, want)
	}
	if got := first.strings[10]; got != "weapon|rare" {
		t.Fatalf("array encoding = %q, want weapon|rare", got)
	}
	if got := first.strings[11]; got != "atk:10|level:2" {
		t.Fatalf("map encoding = %q, want atk:10|level:2", got)
	}
	if got := math.Float32frombits(uint32(first.fixed32[7])); got != float32(1.5) {
		t.Fatalf("ratio = %v, want 1.5", got)
	}
	if got := math.Float64frombits(first.fixed64[8]); got != 3.14159 {
		t.Fatalf("price = %v, want 3.14159", got)
	}
}

// decodedFile 是测试专用的 .bytes 外层结构快照。
// 它只保存断言需要的字段，不作为生产解码器使用。
type decodedFile struct {
	version        uint64
	selfDescribing bool
	keyFieldNo     uint64
	fields         map[uint64]string
	rowCount       uint64
	rows           []decodedRow
}

// decodedRow 是测试专用的单行 TLV 解码结果。
// 按 wire type 分开存储，便于测试直接断言原始 varint/fixed/string 值。
type decodedRow struct {
	varints map[uint64]uint64
	fixed32 map[uint64]uint64
	fixed64 map[uint64]uint64
	strings map[uint64]string
}

// decodeBytesFixture 解码完整 .bytes 文件。
// 解码顺序必须和 binary.go 中 encodeSheet 的写入顺序保持一致。
func decodeBytesFixture(t *testing.T, data []byte) decodedFile {
	t.Helper()
	r := byteReader{data: data}
	if got := string(r.readN(t, len(constants.BytesMagic))); got != constants.BytesMagic {
		t.Fatalf("magic = %q, want %s", got, constants.BytesMagic)
	}
	version := r.uvarint(t)
	_ = r.bytes(t) // schemaHash
	keyFieldNo := r.uvarint(t)
	selfDescribing := r.uvarint(t) != 0
	fieldCount := r.uvarint(t)
	fields := map[uint64]string{}
	for i := uint64(0); i < fieldCount; i++ {
		fieldNo := r.uvarint(t)
		if selfDescribing {
			name := string(r.bytes(t))
			_ = r.bytes(t) // type
			fields[fieldNo] = name
		}
	}
	rowCount := r.uvarint(t)
	rows := make([]decodedRow, 0, rowCount)
	for i := uint64(0); i < rowCount; i++ {
		rows = append(rows, decodeRowFixture(t, r.bytes(t)))
	}
	if r.pos != len(r.data) {
		t.Fatalf("trailing bytes: %d", len(r.data)-r.pos)
	}
	return decodedFile{version: version, selfDescribing: selfDescribing, keyFieldNo: keyFieldNo, fields: fields, rowCount: rowCount, rows: rows}
}

// decodeRowFixture 解码单行 payload。
// 每行 payload 由多个 tag+value 组成，tag 的低 3 位决定接下来要按哪种 wire type 读取。
func decodeRowFixture(t *testing.T, data []byte) decodedRow {
	t.Helper()
	r := byteReader{data: data}
	row := decodedRow{
		varints: map[uint64]uint64{},
		fixed32: map[uint64]uint64{},
		fixed64: map[uint64]uint64{},
		strings: map[uint64]string{},
	}
	for r.pos < len(r.data) {
		tag := r.uvarint(t)
		fieldNo := tag >> 3
		wireType := tag & 7
		switch wireType {
		case 0:
			row.varints[fieldNo] = r.uvarint(t)
		case 1:
			row.fixed64[fieldNo] = binary.LittleEndian.Uint64(r.readN(t, 8))
		case 2:
			row.strings[fieldNo] = string(r.bytes(t))
		case 5:
			row.fixed32[fieldNo] = uint64(binary.LittleEndian.Uint32(r.readN(t, 4)))
		default:
			t.Fatalf("unsupported wire type %d", wireType)
		}
	}
	return row
}

// byteReader 是测试内使用的顺序字节读取器。
// 它维护当前位置，所有读取越界都会直接让测试失败。
type byteReader struct {
	data []byte
	pos  int
}

// uvarint 读取一个无符号 varint。
func (r *byteReader) uvarint(t *testing.T) uint64 {
	t.Helper()
	value, n := binary.Uvarint(r.data[r.pos:])
	if n <= 0 {
		t.Fatalf("invalid uvarint at %d", r.pos)
	}
	r.pos += n
	return value
}

// bytes 读取 length-delimited 数据。
// 格式为先读 varint 长度，再读取对应数量的原始字节。
func (r *byteReader) bytes(t *testing.T) []byte {
	t.Helper()
	size := r.uvarint(t)
	return r.readN(t, int(size))
}

// readN 从当前位置读取固定数量字节。
func (r *byteReader) readN(t *testing.T, n int) []byte {
	t.Helper()
	if r.pos+n > len(r.data) {
		t.Fatalf("read beyond end: pos=%d n=%d len=%d", r.pos, n, len(r.data))
	}
	out := r.data[r.pos : r.pos+n]
	r.pos += n
	return out
}

// unzigzag 把测试中读到的 ZigZag varint 还原为有符号整数。
func unzigzag(value uint64) int64 {
	return int64(value>>1) ^ -int64(value&1)
}

// fixtureUnix 使用和 schema.ConvertValue 相同的本地时区规则计算测试 fixture 中的 datetime 期望值。
func fixtureUnix(t *testing.T, value string) int64 {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Unix()
}

// repoRoot 从当前测试工作目录向上查找 go.mod，定位仓库根目录。
// 这样测试无论从包目录还是仓库根目录启动，都能找到共享 fixture。
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
