// Package app 负责命令行入口的参数解析、流程编排和退出码控制。
//
// 该包是 CLI 的应用层：它不会直接解析 Excel XML 或写具体格式，而是把输入发现、
// XLSX 读取、schema 校验、引用检查、格式转换、.bytes 反解析和代码生成串成一次完整命令执行。
package app

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"iotaexcel/internal/batch"
	"iotaexcel/internal/codegen/cpp"
	"iotaexcel/internal/codegen/csharp"
	"iotaexcel/internal/codegen/golang"
	"iotaexcel/internal/codegen/java"
	"iotaexcel/internal/codegen/javascript"
	"iotaexcel/internal/codegen/python"
	"iotaexcel/internal/constants"
	"iotaexcel/internal/convert"
	"iotaexcel/internal/decode"
	"iotaexcel/internal/logging"
	"iotaexcel/internal/model"
	"iotaexcel/internal/schema"
	"iotaexcel/internal/xlsx"
)

// options 保存 convert/codegen 两类命令的统一参数。
// 即使某些字段只对其中一个命令有效，也保留在同一个结构中，便于 parseOptions 集中校验默认值。
type options struct {
	command   string
	config    string
	input     string
	output    string
	format    string
	recursive bool
	sheet     string
	overwrite bool
	target    string
	checkRef  bool
	strict    bool
	selfDesc  bool
	schemaIn  string
	print     bool
	printMode string
	lang      string
	pkg       string
	logLevel  string
	logFormat string
	logFile   string
}

// Run 是 CLI 的主入口。
// 它接收不包含可执行文件名的参数列表，返回进程退出码，便于 main 和集成测试复用同一套逻辑。
func Run(args []string) int {
	if len(args) == 0 {
		printHelp()
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printHelp()
		return 0
	case "version":
		fmt.Println(constants.ToolVersion)
		return 0
	case "convert", "codegen", "decode":
		return runCommand(args[0], args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		printHelp()
		return 1
	}
}

// runCommand 执行 convert、codegen 或 decode 命令。
// 流程分为三段：
// 1. 解析参数并初始化日志；
// 2. 扫描输入 Excel、读取 xlsx、解析 schema，并按需建立 ref 索引；
// 3. 对每个成功解析的 workbook 执行导出或代码生成，并汇总诊断信息。
func runCommand(command string, args []string) int {
	opts, err := parseOptions(command, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	logger, closeLog, err := logging.New(logging.Config{
		Level:  opts.logLevel,
		Format: opts.logFormat,
		File:   opts.logFile,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer closeLog()

	started := time.Now()
	logger.Info("command started", "command", command, "input", opts.input, "output", opts.output)

	files, err := discoverInputs(opts)
	if err != nil {
		logger.Error("input discovery failed", "command", command, "error", err)
		return 2
	}
	if len(files) == 0 {
		logger.Warn("no input files found", "command", command, "input", opts.input)
		return 0
	}

	if command == "decode" {
		return runDecode(opts, files, logger, started)
	}

	summary := model.Summary{}
	refIndex := schema.NewRefIndex()
	parsed := make([]model.Workbook, 0, len(files))

	for _, file := range files {
		raw, err := xlsx.Read(file.Path, file.RelPath, opts.sheet)
		if err != nil {
			logger.Error("xlsx read failed", "command", command, "source", file.Path, "error", err)
			summary.FailedFiles = append(summary.FailedFiles, file.Path+": "+err.Error())
			continue
		}
		wb, err := schema.ParseWorkbook(raw, schema.Options{Target: opts.target, CheckRef: false})
		if err != nil {
			logger.Error("schema validation failed", "command", command, "source", file.Path, "error", err)
			summary.FailedFiles = append(summary.FailedFiles, file.Path+": "+err.Error())
			continue
		}
		for _, sheet := range wb.Sheets {
			refIndex.Add(sheet.Name, sheet)
		}
		parsed = append(parsed, wb)
	}

	if opts.checkRef {
		for i := range parsed {
			schema.CheckRefs(&parsed[i], refIndex)
		}
	}

	for _, wb := range parsed {
		addStats(&summary, wb)
		if opts.checkRef && workbookHasRefErrors(wb) {
			err := fmt.Errorf("ref validation failed")
			logger.Error("workbook export failed", "command", command, "source", wb.SourcePath, "error", err)
			summary.FailedFiles = append(summary.FailedFiles, wb.SourcePath+": "+err.Error())
			continue
		}
		outputs, err := handleWorkbook(command, opts, wb, logger)
		if err != nil {
			logger.Error("workbook export failed", "command", command, "source", wb.SourcePath, "error", err)
			summary.FailedFiles = append(summary.FailedFiles, wb.SourcePath+": "+err.Error())
			continue
		}
		summary.SuccessFiles = append(summary.SuccessFiles, wb.SourcePath)
		summary.OutputFiles = append(summary.OutputFiles, outputs...)
	}

	logger.Info("summary",
		"command", command,
		"success_files", len(summary.SuccessFiles),
		"failed_files", len(summary.FailedFiles),
		"skipped_files", len(summary.SkippedFiles),
		"type_conversion_errors", summary.TypeConversionErrors,
		"default_value_count", summary.DefaultValueCount,
		"ref_errors", summary.RefErrors,
		"duration_ms", time.Since(started).Milliseconds(),
	)

	if len(summary.FailedFiles) == len(files) {
		return 3
	}
	if len(summary.FailedFiles) > 0 {
		return 4
	}
	return 0
}

// parseOptions 解析命令行参数并补齐默认值。
// 这里会做命令级校验，例如 convert/decode 必须指定 --format，codegen 只允许已支持的目标语言。
func parseOptions(command string, args []string) (options, error) {
	preliminary := defaultOptions(command)
	preliminaryFlags := flag.NewFlagSet(command, flag.ContinueOnError)
	registerOptionFlags(preliminaryFlags, &preliminary)
	if err := preliminaryFlags.Parse(args); err != nil {
		return preliminary, err
	}

	opts := defaultOptions(command)
	if preliminary.config != "" {
		config, err := readConfigOptions(preliminary.config)
		if err != nil {
			return opts, err
		}
		if err := applyConfigOptions(command, &opts, config); err != nil {
			return opts, err
		}
		opts.config = preliminary.config
	}

	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	registerOptionFlags(fs, &opts)
	if err := fs.Parse(args); err != nil {
		return opts, err
	}

	if err := validateOptions(command, opts); err != nil {
		return opts, err
	}
	return opts, nil
}

// defaultOptions 返回所有命令共享的默认参数。
func defaultOptions(command string) options {
	return options{
		command:   command,
		recursive: true,
		target:    "both",
		strict:    true,
		selfDesc:  true,
		printMode: decode.PrintVerbose,
		lang:      constants.CSharpLanguage,
		pkg:       constants.DefaultCSharpNamespace,
		logLevel:  "info",
		logFormat: "text",
	}
}

// registerOptionFlags 把 options 绑定到命令行参数。
func registerOptionFlags(fs *flag.FlagSet, opts *options) {
	fs.StringVar(&opts.config, "config", opts.config, "key=value config file for command options")
	fs.StringVar(&opts.input, "input", opts.input, "input xlsx/.bytes file or directory")
	fs.StringVar(&opts.output, "output", opts.output, "output directory")
	fs.StringVar(&opts.format, "format", opts.format, "output format: csv, json, bin")
	fs.BoolVar(&opts.recursive, "recursive", opts.recursive, "scan input recursively")
	fs.StringVar(&opts.sheet, "sheet", opts.sheet, "sheet name or 1-based index")
	fs.BoolVar(&opts.overwrite, "overwrite", opts.overwrite, "overwrite existing outputs")
	fs.StringVar(&opts.target, "target", opts.target, "target: client, server, both")
	fs.BoolVar(&opts.checkRef, "check-ref", opts.checkRef, "check ref<T> target tables and keys")
	fs.BoolVar(&opts.strict, "strict", opts.strict, "fail current file on schema errors")
	fs.BoolVar(&opts.selfDesc, "self-describing", opts.selfDesc, "include field names and types in .bytes; decode false requires --schema-input")
	fs.StringVar(&opts.schemaIn, "schema-input", opts.schemaIn, "xlsx file or directory used to decode non-self-describing .bytes")
	fs.BoolVar(&opts.print, "print", opts.print, "print decoded .bytes header and rows to stdout")
	fs.StringVar(&opts.printMode, "print-mode", opts.printMode, "decode print mode: verbose or concise")
	fs.StringVar(&opts.lang, "lang", opts.lang, "codegen language")
	fs.StringVar(&opts.pkg, "package", opts.pkg, "package/namespace")
	fs.StringVar(&opts.logLevel, "log-level", opts.logLevel, "debug, info, warn, error")
	fs.StringVar(&opts.logFormat, "log-format", opts.logFormat, "text or json")
	fs.StringVar(&opts.logFile, "log-file", opts.logFile, "optional log file")
}

// readConfigOptions 读取 key=value 参数配置文件。
// 空行会被忽略，行首是 # 的内容会作为注释跳过。
func readConfigOptions(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := map[string]string{}
	lines := strings.Split(string(data), "\n")
	for index, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("read config %s: line %d must be key=value", path, index+1)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("read config %s: line %d has empty key", path, index+1)
		}
		config[key] = strings.TrimSpace(value)
	}
	return config, nil
}

// applyConfigOptions 把配置文件中出现的字段覆盖到默认参数上。
func applyConfigOptions(command string, opts *options, config map[string]string) error {
	for key, value := range config {
		if err := applyConfigValue(command, opts, key, value); err != nil {
			return err
		}
	}
	return nil
}

// applyConfigValue 把单个 key=value 配置项写入 options。
func applyConfigValue(command string, opts *options, key, value string) error {
	switch key {
	case "input":
		opts.input = value
	case "output":
		opts.output = value
	case "convertFormat":
		if command == "convert" {
			opts.format = value
		}
	case "decodeFormat":
		if command == "decode" {
			opts.format = value
		}
	case "recursive":
		parsed, err := parseConfigBool(key, value)
		if err != nil {
			return err
		}
		opts.recursive = parsed
	case "sheet":
		opts.sheet = value
	case "overwrite":
		parsed, err := parseConfigBool(key, value)
		if err != nil {
			return err
		}
		opts.overwrite = parsed
	case "target":
		opts.target = value
	case "checkRef":
		parsed, err := parseConfigBool(key, value)
		if err != nil {
			return err
		}
		opts.checkRef = parsed
	case "strict":
		parsed, err := parseConfigBool(key, value)
		if err != nil {
			return err
		}
		opts.strict = parsed
	case "selfDescribing":
		parsed, err := parseConfigBool(key, value)
		if err != nil {
			return err
		}
		opts.selfDesc = parsed
	case "schemaInput":
		opts.schemaIn = value
	case "print":
		parsed, err := parseConfigBool(key, value)
		if err != nil {
			return err
		}
		opts.print = parsed
	case "printMode":
		opts.printMode = value
	case "lang":
		opts.lang = value
	case "package":
		opts.pkg = value
	case "logLevel":
		opts.logLevel = value
	case "logFormat":
		opts.logFormat = value
	case "logFile":
		opts.logFile = value
	default:
		return fmt.Errorf("unsupported config key %q", key)
	}
	return nil
}

// parseConfigBool 解析配置文件中的 bool 值。
func parseConfigBool(key, value string) (bool, error) {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("config key %q expects bool, got %q", key, value)
	}
	return parsed, nil
}

// validateOptions 做命令级参数校验。
func validateOptions(command string, opts options) error {
	if opts.input == "" {
		return fmt.Errorf("--input is required")
	}
	if opts.output == "" {
		return fmt.Errorf("--output is required")
	}
	if command == "convert" && opts.format == "" {
		return fmt.Errorf("--format is required for convert")
	}
	if command == "convert" && !isOneOf(opts.format, "csv", "json", "bin") {
		return fmt.Errorf("unsupported --format %q", opts.format)
	}
	if command == "decode" && opts.format == "" {
		return fmt.Errorf("--format is required for decode")
	}
	if command == "decode" && !isOneOf(opts.format, "csv", "json") {
		return fmt.Errorf("unsupported decode --format %q", opts.format)
	}
	if command == "decode" && !isOneOf(opts.printMode, decode.PrintVerbose, decode.PrintConcise) {
		return fmt.Errorf("unsupported --print-mode %q", opts.printMode)
	}
	if command == "decode" && !opts.selfDesc && opts.schemaIn == "" {
		return fmt.Errorf("--schema-input is required when decode --self-describing=false")
	}
	if !isOneOf(opts.target, "client", "server", "both") {
		return fmt.Errorf("unsupported --target %q", opts.target)
	}
	if command == "codegen" && !isOneOf(strings.ToLower(opts.lang), constants.CSharpLanguage, constants.GoLanguage, constants.CppLanguage, constants.JavaLanguage, constants.JavaScriptLanguage, constants.PythonLanguage) {
		return fmt.Errorf("unsupported --lang %q", opts.lang)
	}
	return nil
}

// discoverInputs 根据命令类型发现输入文件。
// convert/codegen 处理 .xlsx；decode 处理 .bytes，其他扫描规则保持一致。
func discoverInputs(opts options) ([]batch.File, error) {
	extensions := []string{constants.XlsxExtension}
	if opts.command == "decode" {
		extensions = []string{constants.BytesExtension}
	}
	return batch.Discover(opts.input, batch.Options{Recursive: opts.recursive, Extensions: extensions})
}

// runDecode 执行 .bytes 反解析命令。
// 每个输入 .bytes 会独立输出一个 csv/json 文件，部分文件失败时仍继续处理其他文件。
func runDecode(opts options, files []batch.File, logger *logging.Logger, started time.Time) int {
	summary := model.Summary{}
	schemas, err := loadDecodeSchemas(opts, logger)
	if err != nil {
		logger.Error("decode schema load failed", "command", opts.command, "schema_input", opts.schemaIn, "error", err)
		return 2
	}
	for _, file := range files {
		decoded, err := decode.ReadWithOptions(file.Path, file.RelPath, decode.ReadOptions{Schemas: schemas})
		if err != nil {
			logger.Error("bytes decode failed", "command", opts.command, "source", file.Path, "error", err)
			summary.FailedFiles = append(summary.FailedFiles, file.Path+": "+err.Error())
			continue
		}
		if opts.print {
			decode.Print(os.Stdout, decoded, opts.printMode)
		}
		out, err := decode.Write(decoded, decode.Options{OutputDir: opts.output, Format: opts.format, Overwrite: opts.overwrite})
		if err != nil {
			logger.Error("decode output failed", "command", opts.command, "source", file.Path, "error", err)
			summary.FailedFiles = append(summary.FailedFiles, file.Path+": "+err.Error())
			continue
		}
		summary.SuccessFiles = append(summary.SuccessFiles, file.Path)
		summary.OutputFiles = append(summary.OutputFiles, out)
	}

	logger.Info("summary",
		"command", opts.command,
		"success_files", len(summary.SuccessFiles),
		"failed_files", len(summary.FailedFiles),
		"skipped_files", len(summary.SkippedFiles),
		"duration_ms", time.Since(started).Milliseconds(),
	)
	if len(summary.FailedFiles) == len(files) {
		return 3
	}
	if len(summary.FailedFiles) > 0 {
		return 4
	}
	return 0
}

// loadDecodeSchemas 读取 decode 使用的外部 Excel schema。
// 自描述 .bytes 不需要该参数；非自描述 .bytes 会用 schemaHash 精确匹配这里解析出的 sheet。
func loadDecodeSchemas(opts options, logger *logging.Logger) (map[string][]decode.Field, error) {
	if opts.schemaIn == "" {
		return nil, nil
	}
	files, err := batch.Discover(opts.schemaIn, batch.Options{Recursive: opts.recursive, Extensions: []string{constants.XlsxExtension}})
	if err != nil {
		return nil, err
	}
	out := map[string][]decode.Field{}
	for _, file := range files {
		raw, err := xlsx.Read(file.Path, file.RelPath, opts.sheet)
		if err != nil {
			return nil, err
		}
		wb, err := schema.ParseWorkbook(raw, schema.Options{Target: opts.target, CheckRef: false})
		if err != nil {
			return nil, err
		}
		for _, sheet := range wb.Sheets {
			fields := convert.BinaryFields(sheet.Fields, opts.target)
			decodedFields := make([]decode.Field, 0, len(fields))
			for _, field := range fields {
				decodedFields = append(decodedFields, decode.Field{
					FieldNo: field.FieldNo,
					Name:    field.Name,
					Type:    field.Type,
				})
			}
			out[sheet.SchemaHash] = decodedFields
			logger.Debug("decode schema loaded", "source", file.Path, "sheet", sheet.Name, "schema_hash", sheet.SchemaHash)
		}
	}
	return out, nil
}

// handleWorkbook 根据命令类型把一个已通过 schema 校验的 workbook 写出到目标格式。
// codegen 委托给目标语言生成器；convert 则按 --format 分派到 csv/json/bin 输出器。
func handleWorkbook(command string, opts options, wb model.Workbook, logger *logging.Logger) ([]string, error) {
	if command == "codegen" {
		switch strings.ToLower(opts.lang) {
		case constants.CSharpLanguage:
			return csharp.Generate(wb, csharp.Options{
				OutputDir: opts.output,
				Namespace: opts.pkg,
				Target:    opts.target,
				Overwrite: opts.overwrite,
			})
		case constants.GoLanguage:
			pkg := opts.pkg
			if pkg == constants.DefaultCSharpNamespace {
				pkg = constants.DefaultGoPackage
			}
			return golang.Generate(wb, golang.Options{
				OutputDir: opts.output,
				Package:   pkg,
				Target:    opts.target,
				Overwrite: opts.overwrite,
			})
		case constants.CppLanguage:
			namespace := opts.pkg
			if namespace == constants.DefaultCSharpNamespace {
				namespace = constants.DefaultCppNamespace
			}
			return cpp.Generate(wb, cpp.Options{
				OutputDir: opts.output,
				Namespace: namespace,
				Target:    opts.target,
				Overwrite: opts.overwrite,
			})
		case constants.JavaLanguage:
			pkg := opts.pkg
			if pkg == constants.DefaultCSharpNamespace {
				pkg = constants.DefaultJavaPackage
			}
			return java.Generate(wb, java.Options{
				OutputDir: opts.output,
				Package:   pkg,
				Target:    opts.target,
				Overwrite: opts.overwrite,
			})
		case constants.JavaScriptLanguage:
			return javascript.Generate(wb, javascript.Options{
				OutputDir: opts.output,
				Target:    opts.target,
				Overwrite: opts.overwrite,
			})
		case constants.PythonLanguage:
			return python.Generate(wb, python.Options{
				OutputDir: opts.output,
				Target:    opts.target,
				Overwrite: opts.overwrite,
			})
		default:
			return nil, fmt.Errorf("unsupported --lang %q", opts.lang)
		}
	}

	switch opts.format {
	case "csv":
		return convert.WriteCSV(wb, convert.Options{OutputDir: opts.output, Target: opts.target, Overwrite: opts.overwrite})
	case "json":
		return convert.WriteJSON(wb, convert.Options{OutputDir: opts.output, Target: opts.target, Overwrite: opts.overwrite})
	case "bin":
		return convert.WriteBinary(wb, convert.Options{OutputDir: opts.output, Target: opts.target, Overwrite: opts.overwrite, OmitSelfDescription: !opts.selfDesc})
	default:
		logger.Error("unsupported format", "format", opts.format)
		return nil, fmt.Errorf("unsupported format %q", opts.format)
	}
}

// addStats 把单个 workbook 内的转换错误、默认值使用次数和 ref 错误累加到全局汇总。
func addStats(summary *model.Summary, wb model.Workbook) {
	for _, sheet := range wb.Sheets {
		summary.TypeConversionErrors += len(sheet.ConversionErrors)
		summary.DefaultValueCount += sheet.DefaultValueCount
		summary.RefErrors += len(sheet.RefErrors)
	}
}

// workbookHasRefErrors 判断 workbook 中是否存在引用校验错误。
// 开启 --check-ref 时，只要任意 sheet 有 ref 错误，该 workbook 就不会继续导出。
func workbookHasRefErrors(wb model.Workbook) bool {
	for _, sheet := range wb.Sheets {
		if len(sheet.RefErrors) > 0 {
			return true
		}
	}
	return false
}

// isOneOf 是轻量参数白名单检查，避免为了少量枚举值引入额外依赖。
func isOneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

// printHelp 输出最小帮助文本。
// 详细规则保留在 README 和 docs 中，CLI 内只放最常用的命令示例。
func printHelp() {
	fmt.Println(`iotaexcel converts rule-based Excel config tables.

Usage:
  iotaexcel help
  iotaexcel version
  iotaexcel convert --input ./excels --output ./out --format bin
  iotaexcel convert --input ./excels --output ./out --format bin --self-describing=false
  iotaexcel convert --config ./config.example
  iotaexcel decode --input ./out --output ./decoded --format json
  iotaexcel decode --input ./out --schema-input ./excels --output ./decoded --format json --self-describing=false
  iotaexcel decode --config ./config.example
  iotaexcel codegen --input ./excels --output ./generated --lang csharp
  iotaexcel codegen --input ./excels --output ./generated --lang go
  iotaexcel codegen --input ./excels --output ./generated --lang cpp
  iotaexcel codegen --input ./excels --output ./generated --lang java
  iotaexcel codegen --input ./excels --output ./generated --lang javascript
  iotaexcel codegen --input ./excels --output ./generated --lang python
  iotaexcel codegen --config ./config.example

Commands:
  convert   export Excel sheets to csv, json, or .bytes
  decode    decode .bytes files to csv or json
  codegen   generate reader code
  version   print tool version

Common options:
  --config <file>          read options from a key=value config file
  --input <path>           input Excel/.bytes file or directory
  --output <dir>           output directory
  --format <csv|json|bin>  output format
  --sheet <name|index>     optional sheet filter
  --target <target>        client, server, or both
  --overwrite              overwrite existing outputs
  --log-level <level>      debug, info, warn, or error

Config file options are loaded after defaults, and explicit CLI flags override config values.`)
}
