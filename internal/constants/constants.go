// Package constants 定义跨模块复用的项目级常量。
package constants

const (
	// ToolVersion 是当前 CLI 工具版本。
	ToolVersion = "0.1.0"

	// DefaultCSharpNamespace 是 C# 生成代码默认命名空间。
	DefaultCSharpNamespace = "DataConfig"

	// CSharpLanguage 是 codegen 当前支持的 C# 语言标识。
	CSharpLanguage = "csharp"
)

const (
	// XlsxExtension 是 Excel 输入文件扩展名。
	XlsxExtension = ".xlsx"

	// BytesExtension 是二进制配置文件扩展名。
	BytesExtension = ".bytes"

	// GeneratedCSharpConfigFileSuffix 是 Excel 对应 C# 业务代码文件后缀。
	GeneratedCSharpConfigFileSuffix = ".config.cs"

	// GeneratedCSharpRuntimeFileName 是共享 C# runtime 文件名。
	GeneratedCSharpRuntimeFileName = "IotaExcelRuntime.cs"
)

const (
	// ConfigSuffix 是生成配置类和配置二进制文件名时追加的统一后缀。
	ConfigSuffix = "Config"

	// ConfigTableSuffix 是生成配置表加载类时追加的统一后缀。
	ConfigTableSuffix = "ConfigTable"
)

const (
	// BytesMagic 是 .bytes 文件头魔数，用于快速识别文件格式。
	BytesMagic = "IOTB"

	// BytesFormatVersion 是当前 .bytes 文件格式版本号。
	// 当前开发阶段不做历史版本兼容，因此只维护最新格式版本。
	BytesFormatVersion uint64 = 1
)
