// Package model 定义工具内部流转时使用的统一数据模型。
//
// 该包不包含具体的 Excel 读取、schema 校验或输出逻辑，只承载各阶段共享的数据结构：
// 1. xlsx 包读取出的 RawWorkbook/RawSheet 原始表格数据；
// 2. schema 包解析后的 Workbook/Sheet/Field/Row 结构化数据；
// 3. convert 和 codegen 包输出时依赖的字段类型、用途、wireType 和统计信息。
package model

// Usage 表示 Excel 表头第 3 行声明的字段用途。
// 用途会决定字段是否参与客户端/服务器导出，以及 comment 字段是否跳过二进制写入。
type Usage string

const (
	// UsageClient 表示字段只导出给客户端。
	UsageClient Usage = "client"

	// UsageServer 表示字段只导出给服务器。
	UsageServer Usage = "server"

	// UsageAll 表示字段同时导出给客户端和服务器。
	UsageAll Usage = "all"

	// UsageComment 表示字段仅作为 Excel 备注列，不写入 .bytes，也不参与生成代码字段。
	UsageComment Usage = "comment"
)

// TypeKind 是字段类型的规范化分类。
// 基础类型会直接映射到二进制 wireType 和 C# 类型，泛型类型会额外携带 Inner 或 Args。
type TypeKind string

const (
	TypeBool     TypeKind = "bool"
	TypeInt      TypeKind = "int"
	TypeInt32    TypeKind = "int32"
	TypeInt64    TypeKind = "int64"
	TypeFloat    TypeKind = "float"
	TypeDouble   TypeKind = "double"
	TypeString   TypeKind = "string"
	TypeBytes    TypeKind = "bytes"
	TypeDateTime TypeKind = "datetime"
	TypeArray    TypeKind = "array"
	TypeMap      TypeKind = "map"
	TypeRef      TypeKind = "ref"
)

// TypeSpec 描述一个字段解析后的类型信息。
// Raw 保留规范化后的类型文本，用于 schemaHash 和生成代码；Kind 用于分支处理；
// Args/Inner 保存 array/map/ref 的泛型参数，便于 ref 检查和代码生成。
type TypeSpec struct {
	Raw   string   `json:"raw"`
	Kind  TypeKind `json:"kind"`
	Args  []string `json:"args,omitempty"`
	Inner string   `json:"inner,omitempty"`
}

// Field 描述一个通过前 4 行表头解析出的字段。
// FieldNo 使用 Excel 列号从 1 开始计数，和 protobuf 风格 tag 中的 field number 保持一致。
// WireType 是 .bytes 写入时使用的 protobuf wire type。
type Field struct {
	Name        string   `json:"name"`
	RawName     string   `json:"rawName"`
	Type        TypeSpec `json:"type"`
	Usage       Usage    `json:"usage"`
	Comment     string   `json:"comment"`
	IsKey       bool     `json:"key"`
	Required    bool     `json:"required"`
	Unique      bool     `json:"unique"`
	ColumnIndex int      `json:"columnIndex"`
	FieldNo     uint64   `json:"fieldNo"`
	WireType    uint64   `json:"wireType"`
	Binary      bool     `json:"binary"`
}

// CellValue 保存单元格转换后的值和诊断信息。
// Raw 是原始字符串；Value 是按 TypeSpec 转换后的 Go 值；
// Default 标记该值是否因为空值或转换失败而使用默认值。
type CellValue struct {
	Raw     string `json:"raw"`
	Value   any    `json:"value"`
	Default bool   `json:"default"`
	Error   string `json:"error,omitempty"`
}

// Row 表示一行有效数据。
// Index 保存 Excel 中的 1-based 行号，Key 保存唯一 key 的原始文本，Values 按字段名索引。
type Row struct {
	Index  int                  `json:"row"`
	Key    string               `json:"key"`
	Values map[string]CellValue `json:"values"`
}

// Sheet 是 schema 校验后的 sheet 结构。
// 它同时包含可导出的字段、有效数据行、schemaHash，以及转换/引用校验期间产生的诊断信息。
type Sheet struct {
	Name              string   `json:"name"`
	Fields            []Field  `json:"fields"`
	Rows              []Row    `json:"rows"`
	SchemaHash        string   `json:"schemaHash,omitempty"`
	ConversionErrors  []string `json:"conversionErrors,omitempty"`
	DefaultValueCount int      `json:"defaultValueCount"`
	SkippedEmptyRows  []int    `json:"skippedEmptyRows,omitempty"`
	RefErrors         []string `json:"refErrors,omitempty"`
}

// Workbook 是一个 Excel 文件解析后的结构化表示。
// RelPath 用于批量导出时保留输入目录下的相对路径。
type Workbook struct {
	SourcePath string  `json:"source"`
	RelPath    string  `json:"relPath"`
	Sheets     []Sheet `json:"sheets"`
}

// RawSheet 是 xlsx 包直接从工作簿 XML 读出的原始 sheet。
// Rows 只包含字符串矩阵，还没有进行字段、类型、用途或 key 规则校验。
type RawSheet struct {
	Name string
	Rows [][]string
}

// RawWorkbook 是 xlsx 包的读取结果。
// schema 包会基于该结构继续解析表头和数据行。
type RawWorkbook struct {
	SourcePath string
	RelPath    string
	Sheets     []RawSheet
}

// Summary 聚合一次命令执行过程中的成功、失败、输出和诊断数量。
// 当前主要用于 CLI 结束时输出结构化日志。
type Summary struct {
	SuccessFiles         []string `json:"successFiles"`
	FailedFiles          []string `json:"failedFiles"`
	SkippedFiles         []string `json:"skippedFiles"`
	OutputFiles          []string `json:"outputFiles"`
	TypeConversionErrors int      `json:"typeConversionErrors"`
	DefaultValueCount    int      `json:"defaultValueCount"`
	RefErrors            int      `json:"refErrors"`
}
