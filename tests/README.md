# 测试

该目录用于存放 `iotaexcel` 的集成测试 fixture 和预期输出。

## 目录结构

- `testdata/excels/`：源 `.xlsx` fixture。
- `testdata/expected/csv/`：预期 CSV 输出。
- `testdata/expected/json/`：预期 JSON 输出。
- `testdata/expected/bytes/`：预期 `.bytes` 输出或解码快照。
- `testdata/expected/csharp/`：预期生成的 C# 文件。
- `testdata/expected/go/`：预期生成的 Go 文件。
- `testdata/expected/cpp/`：预期生成的 C++ 文件。

单元测试应放在对应 Go 包旁边。跨包集成测试可以使用该目录作为共享测试数据。

## Fixture 生成

运行以下脚本重新生成 `.xlsx` fixture：

```powershell
powershell -ExecutionPolicy Bypass -File tests/generate-fixtures.ps1
```

```bash
sh tests/generate-fixtures.sh
```

如需一键重新生成 fixture，导出自描述/非自描述 `.bytes`，反解析 CSV/JSON，并生成 C#、Go 和 C++ 代码，可运行：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/export-test-fixtures.ps1
```

```bash
sh scripts/export-test-fixtures.sh
```

输出目录：

- `.bytes`：`out/bytes`
- 反解析 CSV：`out/decoded-csv`
- 反解析 JSON：`out/decoded-json`
- 非自描述 `.bytes`：`out/bytes-compact`
- 非自描述反解析 CSV：`out/decoded-csv-compact`
- 非自描述反解析 JSON：`out/decoded-json-compact`
- C#：`out/codegen/csharp`
- Go：`out/codegen/go`
- C++：`out/codegen/cpp`

如需只验证 `decode` 命令，可运行：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/decode-test-fixtures.ps1
```

```bash
sh scripts/decode-test-fixtures.sh
```

`decode` 测试输出目录：

- `.bytes`：`out/decode-test/bytes`
- JSON：`out/decode-test/json`
- CSV：`out/decode-test/csv`
- 非自描述 `.bytes`：`out/decode-test/bytes-compact`
- 非自描述 JSON：`out/decode-test/json-compact`
- 非自描述 CSV：`out/decode-test/csv-compact`
- 打印日志：`out/decode-test/decode-print.txt`

所有 PowerShell 脚本执行结束后都会等待按 Enter 退出。自动化环境可设置 `IOTAEXCEL_NO_PAUSE=1` 跳过等待。

## Excel Fixture

- `valid/Config.xlsx`：主要合法工作簿，覆盖多 sheet、sharedStrings、key 标记、用途别名、基础类型、负整数、datetime、array、map、comment 字段、空行跳过和 `ref<Item>`。
- `valid/InlineStrings.xlsx`：使用 `inlineStr` 单元格的合法工作簿。
- `valid/Defaults.xlsx`：包含非法单元格值的合法工作簿，用于验证转换错误日志和默认值处理。
- `nested/SubConfig.xlsx`：嵌套目录下的合法工作簿，用于验证输出路径保留相对目录结构。
- `invalid/MissingKey.xlsx`：缺少唯一 key 标记。
- `invalid/DuplicateKey.xlsx`：key 值重复。
- `invalid/InvalidFieldName.xlsx`：字段标识符非法。
- `invalid/InvalidUsage.xlsx`：字段用途不支持。
- `invalid/InvalidType.xlsx`：字段类型不支持。
- `invalid/CommentKey.xlsx`：key 字段被标记为 `comment`。
- `invalid/EmptyKey.xlsx`：数据行 key 为空。
- `invalid/Invalid File.xlsx`：用于代码生成的 Excel 文件名非法。
- `invalid/InvalidSheetName.xlsx`：用于 C# 类生成的 sheet 名非法。
- `invalid/RefMissing.xlsx`：开启 `--check-ref` 时，`ref<Item>` 目标表缺失。
- `~$Temp.xlsx`：应被扫描流程跳过的 Excel 临时文件 fixture。
