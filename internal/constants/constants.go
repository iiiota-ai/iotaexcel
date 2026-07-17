// Package constants defines project-wide constants shared across modules.
package constants

const (
	// ToolVersion is the current CLI tool version.
	ToolVersion = "0.1.0"

	// DefaultCSharpNamespace is the default namespace for generated C# code.
	DefaultCSharpNamespace = "DataConfig"

	// DefaultGoPackage is the default package name for generated Go code.
	DefaultGoPackage = "dataconfig"

	// DefaultCppNamespace is the default namespace for generated C++ code.
	DefaultCppNamespace = "DataConfig"

	// DefaultJavaPackage is the default package name for generated Java code.
	DefaultJavaPackage = "dataconfig"

	// CSharpLanguage is the codegen language identifier for C#.
	CSharpLanguage = "csharp"

	// GoLanguage is the codegen language identifier for Go.
	GoLanguage = "go"

	// CppLanguage is the codegen language identifier for C++.
	CppLanguage = "cpp"

	// JavaLanguage is the codegen language identifier for Java.
	JavaLanguage = "java"

	// JavaScriptLanguage is the codegen language identifier for JavaScript.
	JavaScriptLanguage = "javascript"

	// PythonLanguage is the codegen language identifier for Python.
	PythonLanguage = "python"

	// SwiftLanguage is the codegen language identifier for Swift.
	SwiftLanguage = "swift"
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

	// GeneratedCppConfigFileSuffix is the generated C++ config header suffix.
	GeneratedCppConfigFileSuffix = ".config.hpp"

	// GeneratedCppRuntimeFileName is the shared C++ runtime header name.
	GeneratedCppRuntimeFileName = "iotaexcel_runtime.hpp"

	// GeneratedJavaConfigFileSuffix is the generated Java config file suffix.
	GeneratedJavaConfigFileSuffix = ".java"

	// GeneratedJavaRuntimeFileName is the shared Java runtime file name.
	GeneratedJavaRuntimeFileName = "IotaExcelRuntime.java"

	// GeneratedJavaScriptConfigFileSuffix is the generated JavaScript config file suffix.
	GeneratedJavaScriptConfigFileSuffix = ".config.js"

	// GeneratedJavaScriptRuntimeFileName is the shared JavaScript runtime file name.
	GeneratedJavaScriptRuntimeFileName = "iotaexcel_runtime.js"

	// GeneratedPythonConfigFileSuffix is the generated Python config file suffix.
	GeneratedPythonConfigFileSuffix = "_config.py"

	// GeneratedPythonRuntimeFileName is the shared Python runtime file name.
	GeneratedPythonRuntimeFileName = "iotaexcel_runtime.py"

	// GeneratedSwiftConfigFileSuffix is the generated Swift config file suffix.
	GeneratedSwiftConfigFileSuffix = ".config.swift"

	// GeneratedSwiftRuntimeFileName is the shared Swift runtime file name.
	GeneratedSwiftRuntimeFileName = "IotaExcelRuntime.swift"
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
