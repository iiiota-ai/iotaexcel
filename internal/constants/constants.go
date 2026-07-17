// Package constants defines project-wide constants shared across modules.
package constants

const (
	// ToolVersion is the current CLI tool version.
	ToolVersion = "0.1.0"

	// DefaultCSharpNamespace is the default namespace for generated C# code.
	DefaultCSharpNamespace = "DataConfig"

	// DefaultGoPackage is the default package name for generated Go code.
	DefaultGoPackage = "dataconfig"

	// CSharpLanguage is the codegen language identifier for C#.
	CSharpLanguage = "csharp"

	// GoLanguage is the codegen language identifier for Go.
	GoLanguage = "go"
)

const (
	// XlsxExtension is the Excel input file extension.
	XlsxExtension = ".xlsx"

	// BytesExtension is the binary config file extension.
	BytesExtension = ".bytes"

	// GeneratedCSharpConfigFileSuffix is the generated C# config file suffix.
	GeneratedCSharpConfigFileSuffix = ".config.cs"

	// GeneratedCSharpRuntimeFileName is the shared C# runtime file name.
	GeneratedCSharpRuntimeFileName = "IotaExcelRuntime.cs"

	// GeneratedGoConfigFileSuffix is the generated Go config file suffix.
	GeneratedGoConfigFileSuffix = ".config.go"

	// GeneratedGoRuntimeFileName is the shared Go runtime file name.
	GeneratedGoRuntimeFileName = "iotaexcel_runtime.go"
)

const (
	// ConfigSuffix is appended to generated config type and bytes file names.
	ConfigSuffix = "Config"

	// ConfigTableSuffix is appended to generated table loader type names.
	ConfigTableSuffix = "ConfigTable"
)

const (
	// BytesMagic identifies .bytes files.
	BytesMagic = "IOTB"

	// BytesFormatVersion is the current .bytes format version.
	BytesFormatVersion uint64 = 1
)
