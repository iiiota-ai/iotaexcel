package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"iotaexcel/internal/batch"
	"iotaexcel/internal/constants"
)

// TestIntegrationValidConvertAndCodegen 覆盖主合法 fixture 的核心链路：
// .bytes 导出、JSON 调试输出和 C# 代码生成都应该成功。
func TestIntegrationValidConvertAndCodegen(t *testing.T) {
	root := repoRoot(t)
	input := filepath.Join(root, "tests", "testdata", "excels", "valid")
	config := filepath.Join(input, "Config.xlsx")

	binOut := t.TempDir()
	codeOut := t.TempDir()
	goCodeOut := t.TempDir()
	cppCodeOut := t.TempDir()
	jsonOut := t.TempDir()

	exit := Run([]string{
		"convert",
		"--input", config,
		"--output", binOut,
		"--format", "bin",
		"--check-ref",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("valid bin convert exit = %d, want 0", exit)
	}
	itemBytes := assertFile(t, binOut, "Config_Item"+constants.ConfigSuffix+constants.BytesExtension)
	assertFile(t, binOut, "Config_Hero"+constants.ConfigSuffix+constants.BytesExtension)

	decodeJSONOut := t.TempDir()
	exit = Run([]string{
		"decode",
		"--input", binOut,
		"--output", decodeJSONOut,
		"--format", "json",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("valid decode json exit = %d, want 0", exit)
	}
	decodedJSON, err := os.ReadFile(assertFile(t, decodeJSONOut, "Config_ItemConfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(decodedJSON), `"name": "Sword"`) {
		t.Fatalf("decoded JSON does not contain Sword row")
	}
	expectedCreatedAt := strconv.FormatInt(fixtureUnix(t, "2026-07-10 18:47:00"), 10)
	if !strings.Contains(string(decodedJSON), `"createdAt": `+expectedCreatedAt) {
		t.Fatalf("decoded JSON does not contain datetime seconds")
	}

	decodeCSVOut := t.TempDir()
	exit = Run([]string{
		"decode",
		"--input", itemBytes,
		"--output", decodeCSVOut,
		"--format", "csv",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("valid decode csv exit = %d, want 0", exit)
	}
	decodedCSV, err := os.ReadFile(assertFile(t, decodeCSVOut, "Config_ItemConfig.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(decodedCSV), "Sword") {
		t.Fatalf("decoded CSV does not contain Sword row")
	}

	slimBytesOut := t.TempDir()
	exit = Run([]string{
		"convert",
		"--input", config,
		"--output", slimBytesOut,
		"--format", "bin",
		"--self-describing=false",
		"--check-ref",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("non-self-describing bin convert exit = %d, want 0", exit)
	}
	slimDecodeOut := t.TempDir()
	exit = Run([]string{
		"decode",
		"--input", slimBytesOut,
		"--schema-input", config,
		"--self-describing=false",
		"--output", slimDecodeOut,
		"--format", "json",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("non-self-describing decode json exit = %d, want 0", exit)
	}
	slimDecodedJSON, err := os.ReadFile(assertFile(t, slimDecodeOut, "Config_ItemConfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(slimDecodedJSON), `"name": "Sword"`) {
		t.Fatalf("non-self-describing decoded JSON does not contain Sword row")
	}

	exit = Run([]string{
		"convert",
		"--input", input,
		"--output", jsonOut,
		"--format", "json",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("valid json convert exit = %d, want 0", exit)
	}
	assertFile(t, jsonOut, "Config_Item.json")

	exit = Run([]string{
		"codegen",
		"--input", config,
		"--output", codeOut,
		"--lang", constants.CSharpLanguage,
		"--check-ref",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("valid codegen exit = %d, want 0", exit)
	}
	generated := assertFile(t, codeOut, "Config"+constants.GeneratedCSharpConfigFileSuffix)
	runtime := assertFile(t, codeOut, constants.GeneratedCSharpRuntimeFileName)
	content, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}
	runtimeContent, err := os.ReadFile(runtime)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "namespace "+constants.DefaultCSharpNamespace) {
		t.Fatalf("generated C# does not contain DataConfig namespace")
	}
	if !strings.Contains(string(content), "public sealed class ItemConfig") {
		t.Fatalf("generated C# does not contain ItemConfig class")
	}
	if !strings.Contains(string(content), "public sealed class ItemConfigTable") {
		t.Fatalf("generated C# does not contain ItemConfigTable class")
	}
	if !strings.Contains(string(content), "private readonly IReadOnlyDictionary<int, ItemConfig> datas") {
		t.Fatalf("generated C# does not contain datas field")
	}
	if strings.Contains(string(content), "GetById") {
		t.Fatalf("generated C# should not contain unsafe GetById method")
	}
	if !strings.Contains(string(content), "public bool TryGetByid(int key, out ItemConfig value)") {
		t.Fatalf("generated C# does not contain TryGetByid method")
	}
	if !strings.Contains(string(content), "using System.Threading.Tasks;") {
		t.Fatalf("generated C# does not import Tasks namespace")
	}
	if !strings.Contains(string(content), "public static async Task<ItemConfigTable> LoadAsync(Func<string, Task<byte[]>> readBytesAsync)") {
		t.Fatalf("generated C# does not contain async table loader")
	}
	if !strings.Contains(string(content), `readBytesAsync("Config_Item`+constants.ConfigSuffix+constants.BytesExtension+`")`) {
		t.Fatalf("generated C# async loader does not use expected bytes file name")
	}
	if strings.Contains(string(content), "internal sealed class IotaBytesReader") {
		t.Fatalf("generated C# should not embed shared TLV reader")
	}
	if !strings.Contains(string(runtimeContent), "ReadVarUInt64") {
		t.Fatalf("runtime C# does not contain TLV reader")
	}
	if strings.Contains(string(content), "TODO: implement protobuf-style TLV reader") {
		t.Fatalf("generated C# still contains TLV reader TODO")
	}

	exit = Run([]string{
		"codegen",
		"--input", config,
		"--output", goCodeOut,
		"--lang", constants.GoLanguage,
		"--check-ref",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("valid go codegen exit = %d, want 0", exit)
	}
	goGenerated := assertFile(t, goCodeOut, "Config"+constants.GeneratedGoConfigFileSuffix)
	goRuntime := assertFile(t, goCodeOut, constants.GeneratedGoRuntimeFileName)
	goContent, err := os.ReadFile(goGenerated)
	if err != nil {
		t.Fatal(err)
	}
	goRuntimeContent, err := os.ReadFile(goRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goContent), "package "+constants.DefaultGoPackage) {
		t.Fatalf("generated Go does not contain default package")
	}
	if !strings.Contains(string(goContent), "type ItemConfig struct") {
		t.Fatalf("generated Go does not contain ItemConfig struct")
	}
	if !strings.Contains(string(goContent), "type ItemConfigTable struct") {
		t.Fatalf("generated Go does not contain ItemConfigTable struct")
	}
	if !strings.Contains(string(goContent), "datas map[int32]*ItemConfig") {
		t.Fatalf("generated Go does not contain typed datas map")
	}
	if !strings.Contains(string(goContent), "func (t *ItemConfigTable) TryGetByid(key int32) (*ItemConfig, bool)") {
		t.Fatalf("generated Go does not contain TryGetByid method")
	}
	if !strings.Contains(string(goContent), "func LoadItemConfigTable(data []byte) (*ItemConfigTable, error)") {
		t.Fatalf("generated Go does not contain LoadItemConfigTable")
	}
	if !strings.Contains(string(goContent), `readBytes("Config_Item`+constants.ConfigSuffix+constants.BytesExtension+`")`) {
		t.Fatalf("generated Go loader does not use expected bytes file name")
	}
	if !strings.Contains(string(goRuntimeContent), "func (r *iotaBytesReader) readVarUint64()") {
		t.Fatalf("runtime Go does not contain TLV reader")
	}

	exit = Run([]string{
		"codegen",
		"--input", config,
		"--output", cppCodeOut,
		"--lang", constants.CppLanguage,
		"--check-ref",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("valid cpp codegen exit = %d, want 0", exit)
	}
	cppGenerated := assertFile(t, cppCodeOut, "Config"+constants.GeneratedCppConfigFileSuffix)
	cppRuntime := assertFile(t, cppCodeOut, constants.GeneratedCppRuntimeFileName)
	cppContent, err := os.ReadFile(cppGenerated)
	if err != nil {
		t.Fatal(err)
	}
	cppRuntimeContent, err := os.ReadFile(cppRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cppContent), "namespace "+constants.DefaultCppNamespace) {
		t.Fatalf("generated C++ does not contain default namespace")
	}
	if !strings.Contains(string(cppContent), "struct ItemConfig") {
		t.Fatalf("generated C++ does not contain ItemConfig struct")
	}
	if !strings.Contains(string(cppContent), "class ItemConfigTable") {
		t.Fatalf("generated C++ does not contain ItemConfigTable class")
	}
	if !strings.Contains(string(cppContent), "using DataMap = std::unordered_map<KeyType, ItemConfig>") {
		t.Fatalf("generated C++ does not contain typed datas map")
	}
	if !strings.Contains(string(cppContent), "bool TryGetByid(const KeyType& key, const ItemConfig*& value) const") {
		t.Fatalf("generated C++ does not contain TryGetByid method")
	}
	if !strings.Contains(string(cppContent), "static ItemConfigTable Load(const std::vector<std::uint8_t>& data)") {
		t.Fatalf("generated C++ does not contain Load")
	}
	if !strings.Contains(string(cppContent), `readBytes("Config_Item`+constants.ConfigSuffix+constants.BytesExtension+`")`) {
		t.Fatalf("generated C++ loader does not use expected bytes file name")
	}
	if !strings.Contains(string(cppRuntimeContent), "std::uint64_t ReadVarUInt64()") {
		t.Fatalf("runtime C++ does not contain TLV reader")
	}
}

// TestIntegrationConfigFileOptions 验证命令可以从 key=value 配置文件读取参数。
// 配置文件适合保存批处理默认值；命令行显式参数仍然可以覆盖配置文件。
func TestIntegrationConfigFileOptions(t *testing.T) {
	root := repoRoot(t)
	configXLSX := filepath.Join(root, "tests", "testdata", "excels", "valid", "Config.xlsx")

	bytesOut := t.TempDir()
	convertConfig := filepath.Join(t.TempDir(), "convert.conf")
	writeTestFile(t, convertConfig, "# convert 输入 Excel。\n"+
		"input="+configXLSX+"\n"+
		"output="+bytesOut+"\n"+
		"convertFormat=bin\n"+
		"sheet=Item\n"+
		"# 启用 ref<T> 校验，验证注释行会被配置解析器跳过。\n"+
		"checkRef=true\n"+
		"selfDescribing=false\n"+
		"overwrite=true\n"+
		"logLevel=error\n")
	exit := Run([]string{"convert", "--config", convertConfig})
	if exit != 0 {
		t.Fatalf("convert with config exit = %d, want 0", exit)
	}
	assertFile(t, bytesOut, "Config_Item"+constants.ConfigSuffix+constants.BytesExtension)

	decodeOut := t.TempDir()
	decodeConfig := filepath.Join(t.TempDir(), "decode.conf")
	writeTestFile(t, decodeConfig, "input="+bytesOut+"\n"+
		"schemaInput="+configXLSX+"\n"+
		"selfDescribing=false\n"+
		"output="+decodeOut+"\n"+
		"decodeFormat=json\n"+
		"overwrite=true\n"+
		"logLevel=error\n")
	exit = Run([]string{"decode", "--config", decodeConfig})
	if exit != 0 {
		t.Fatalf("decode with config exit = %d, want 0", exit)
	}
	decodedJSON, err := os.ReadFile(assertFile(t, decodeOut, "Config_ItemConfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(decodedJSON), `"selfDescribing": false`) {
		t.Fatalf("decoded JSON does not reflect config selfDescribing=false")
	}

	overrideOut := t.TempDir()
	exit = Run([]string{
		"convert",
		"--config", convertConfig,
		"--output", overrideOut,
		"--format", "json",
	})
	if exit != 0 {
		t.Fatalf("convert with config override exit = %d, want 0", exit)
	}
	assertFile(t, overrideOut, "Config_Item.json")
}

// TestIntegrationNestedAndTempFileDiscovery 验证目录扫描行为。
// 嵌套目录应保留输出结构，Excel 临时文件（~$ 前缀）不应被 Discover 返回。
func TestIntegrationNestedAndTempFileDiscovery(t *testing.T) {
	root := repoRoot(t)
	out := t.TempDir()

	exit := Run([]string{
		"convert",
		"--input", filepath.Join(root, "tests", "testdata", "excels", "nested"),
		"--output", out,
		"--format", "bin",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("nested fixture convert exit = %d, want 0", exit)
	}
	assertFile(t, out, "SubConfig_SubConfig"+constants.ConfigSuffix+constants.BytesExtension)

	files, err := batch.Discover(filepath.Join(root, "tests", "testdata", "excels"), batch.Options{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasPrefix(filepath.Base(file.Path), "~$") {
			t.Fatalf("temp Excel file was discovered: %s", file.Path)
		}
	}
}

// TestIntegrationDefaultsFixtureConvertsWithDefaults 验证非法单元格值会回退默认值。
// Defaults.xlsx 是合法 schema，但包含无法转换的 bool/int/datetime 单元格。
func TestIntegrationDefaultsFixtureConvertsWithDefaults(t *testing.T) {
	root := repoRoot(t)
	input := filepath.Join(root, "tests", "testdata", "excels", "valid", "Defaults.xlsx")
	out := t.TempDir()

	exit := Run([]string{
		"convert",
		"--input", input,
		"--output", out,
		"--format", "json",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("Defaults.xlsx convert exit = %d, want 0", exit)
	}
	content, err := os.ReadFile(assertFile(t, out, "Defaults_Defaults.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"defaultValueCount"`) {
		t.Fatalf("Defaults JSON does not include defaultValueCount")
	}
}

// TestIntegrationInlineStringsFixtureConverts 验证 inlineStr 单元格读取。
// 这是对 sharedStrings 之外的另一种常见 XLSX 字符串存储方式的覆盖。
func TestIntegrationInlineStringsFixtureConverts(t *testing.T) {
	root := repoRoot(t)
	input := filepath.Join(root, "tests", "testdata", "excels", "valid", "InlineStrings.xlsx")
	out := t.TempDir()

	exit := Run([]string{
		"convert",
		"--input", input,
		"--output", out,
		"--format", "bin",
		"--overwrite",
		"--log-level", "error",
	})
	if exit != 0 {
		t.Fatalf("InlineStrings.xlsx convert exit = %d, want 0", exit)
	}
	assertFile(t, out, "InlineStrings_Inline"+constants.ConfigSuffix+constants.BytesExtension)
}

// TestIntegrationInvalidFixturesFail 确认典型非法 Excel 会让当前文件导出失败。
// 这些 fixture 分别覆盖缺 key、重复 key、命名非法、用途非法、类型非法等 schema 错误。
func TestIntegrationInvalidFixturesFail(t *testing.T) {
	root := repoRoot(t)
	cases := []string{
		"MissingKey.xlsx",
		"DuplicateKey.xlsx",
		"InvalidFieldName.xlsx",
		"InvalidUsage.xlsx",
		"InvalidType.xlsx",
		"CommentKey.xlsx",
		"EmptyKey.xlsx",
		"Invalid File.xlsx",
		"InvalidSheetName.xlsx",
	}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			input := filepath.Join(root, "tests", "testdata", "excels", "invalid", name)
			exit := Run([]string{
				"convert",
				"--input", input,
				"--output", t.TempDir(),
				"--format", "bin",
				"--overwrite",
				"--log-level", "error",
			})
			if exit == 0 {
				t.Fatalf("%s exit = 0, want failure", name)
			}
		})
	}
}

// TestIntegrationRefCheckFailsMissingReference 验证 --check-ref 会检查引用目标表和 key。
// 该 fixture 的 ref<Item> 目标表不存在，因此开启引用检查后必须失败。
func TestIntegrationRefCheckFailsMissingReference(t *testing.T) {
	root := repoRoot(t)
	input := filepath.Join(root, "tests", "testdata", "excels", "invalid", "RefMissing.xlsx")
	exit := Run([]string{
		"convert",
		"--input", input,
		"--output", t.TempDir(),
		"--format", "bin",
		"--check-ref",
		"--overwrite",
		"--log-level", "error",
	})
	if exit == 0 {
		t.Fatalf("RefMissing.xlsx with --check-ref exit = 0, want failure")
	}
}

// writeTestFile 写入测试用临时文件，失败时直接终止当前测试。
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// repoRoot 从测试运行目录向上查找 go.mod，得到仓库根目录。
// 集成测试依赖 tests/testdata 下的共享 fixture，因此不能假设当前工作目录固定。
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

// assertFile 断言指定相对路径存在且不是目录，并返回完整路径供后续读取。
func assertFile(t *testing.T, root, rel string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("expected file %s, got directory", path)
	}
	return path
}

// assertMissing 断言指定相对路径不存在。
// 当前保留该 helper 供后续补充负向输出检查时复用。
func assertMissing(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, err=%v", path, err)
	}
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
