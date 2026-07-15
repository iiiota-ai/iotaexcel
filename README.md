# iotaexcel

`iotaexcel` 是一个跨平台命令行工具，用于把规则化 Excel 配置表转换为 `.bytes` 二进制文件和 C# 读取代码。

## 环境要求

- 从源码构建需要 Go 1.19 或更高版本。

## 技术选型

本项目选择 Go 实现 CLI，主要考虑：

- 跨平台构建能力强。同一套源码可以直接交叉编译出 Windows、Linux 和 macOS 可执行文件。
- 产物形态简单。Go 可以生成单个独立可执行文件，方便在工具链、CI 或策划本地环境中分发使用。
- 包体相对可控。当前构建使用 `CGO_ENABLED=0`、`-trimpath`、`-ldflags="-s -w"` 减少运行时依赖和调试符号体积。
- 部署依赖少。当前项目没有引入第三方 Go module，核心能力基于标准库和项目内实现完成。
- 适合命令行批处理。Excel 扫描、格式校验、`.bytes` 编码、代码生成、日志输出都属于偏 I/O 和批处理的任务，Go 的标准库能较好覆盖。

对应取舍是：为了保持独立可执行文件和低依赖，当前 XLSX 读取、`.iotaignore` 解析、protobuf 风格 TLV 编解码和 C# 代码生成都由项目内代码实现，而不是依赖大型框架或外部运行时。

## 构建

Go 构建只会编译 `./cmd/iotaexcel` 及其正常依赖，`*_test.go` 测试代码和 `tests/testdata` fixture 不会被打包进可执行文件。当前项目没有使用 `go:embed` 嵌入测试资源。

Windows:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build.ps1
powershell -ExecutionPolicy Bypass -File scripts/build.ps1 -All
```

Linux/macOS:

```bash
sh scripts/build.sh
sh scripts/build.sh --all
```

构建产物输出到 `dist/`，默认使用 `CGO_ENABLED=0`、`-trimpath`、`-ldflags="-s -w"` 生成更小的独立可执行文件。构建脚本会同时生成 `dist/sha256sums.txt`，用于发布后校验下载到的可执行文件是否完整。

构建脚本会把 Go 构建缓存放到项目内的 `.gocache/`，避免本机默认 Go cache 状态影响构建结果。该目录已加入 `.gitignore`。

## 验证

提交前建议运行文档检查和 Go 测试：

Windows:

```powershell
$env:IOTAEXCEL_NO_PAUSE = "1"
powershell -ExecutionPolicy Bypass -File scripts/check-docs.ps1
powershell -ExecutionPolicy Bypass -File scripts/test.ps1
```

Linux/macOS:

```bash
sh scripts/check-docs.sh
sh scripts/test.sh
```

`scripts/test.ps1` 会设置 UTF-8 输出，并把 `GOCACHE` 指向项目内 `.gocache/` 后执行 `go test ./...`。如果直接双击运行 PowerShell 脚本，默认会在结束前暂停；设置 `IOTAEXCEL_NO_PAUSE=1` 可用于 CI 或自动化脚本。

GitHub Actions 会在 push 和 pull request 时执行格式检查、文档检查、Go 测试，并运行 `scripts/build.sh --all` 验证多平台发布产物可以正常生成。

推送 `v*` tag 时，GitHub Actions 会自动创建 GitHub Release，并上传 `dist/` 下的多平台可执行文件和 `sha256sums.txt`。示例：

```bash
git tag v0.1.0
git push origin v0.1.0
```

## 快速开始

### 导出 `.bytes`

```bash
iotaexcel convert --input ./excels --output ./out --format bin
iotaexcel convert --input ./excels --output ./out --format bin --self-describing=false
iotaexcel convert --input ./excels --output ./out --format json
iotaexcel convert --config ./iotaexcel.convert.conf
```

参数说明：

- `convert`：读取 Excel，并导出为指定格式。
- `--input ./excels`：输入 Excel 文件或目录。
- `--output ./out`：输出目录。
- `--format bin`：导出 `.bytes` 二进制文件。
- `--format json`：导出 JSON 调试文件。
- `--self-describing=false`：导出非自描述 `.bytes`，不写入字段名和类型名，文件体积更小。
- `--config ./iotaexcel.convert.conf`：从 `key=value` 配置文件读取参数，命令行显式传入的参数会覆盖配置文件。

### 反解析 `.bytes`

```bash
iotaexcel decode --input ./out --output ./decoded --format json
iotaexcel decode --input ./out --schema-input ./excels --output ./decoded --format json --self-describing=false
iotaexcel decode --input ./out --output ./decoded --format json --print
iotaexcel decode --input ./out --output ./decoded --format json --print --print-mode concise
iotaexcel decode --config ./iotaexcel.decode.conf
```

参数说明：

- `decode`：读取 `.bytes`，反解析为 CSV 或 JSON。
- `--input ./out`：输入 `.bytes` 文件或目录。
- `--output ./decoded`：反解析输出目录。
- `--format json`：输出 JSON；也可使用 `--format csv` 输出 CSV。
- `--schema-input ./excels`：非自描述 `.bytes` 必需，指向原 Excel schema 文件或目录。
- `--self-describing=false`：告诉 decode 当前 `.bytes` 不包含字段名和类型名，需要配合 `--schema-input` 使用。
- `--print`：除写出文件外，同时把 `.bytes` 头部、字段和行数据打印到终端。
- `--print-mode concise`：打印简洁模式，只输出字面量和 TLV 数字，便于脚本比对。
- `--config ./iotaexcel.decode.conf`：从 `key=value` 配置文件读取 decode 参数。

### 生成 C# 读取代码

```bash
iotaexcel codegen --input ./excels --output ./generated --lang csharp
iotaexcel codegen --config ./iotaexcel.codegen.conf
```

参数说明：

- `codegen`：根据 Excel schema 生成读取 `.bytes` 的代码。
- `--input ./excels`：输入 Excel 文件或目录。
- `--output ./generated`：生成代码输出目录。
- `--lang csharp`：目标语言，当前优先支持 C#。
- `--config ./iotaexcel.codegen.conf`：从 `key=value` 配置文件读取 codegen 参数。

codegen 会为每个 Excel 生成一个 `Excel名.config.cs` 业务配置文件，并额外生成共享 runtime 文件 `IotaExcelRuntime.cs`。业务文件中每个 sheet 会生成 `Sheet名Config` 数据类和 `Sheet名ConfigTable` loader 类，按唯一 key 生成 `TryGetBy<Key字段名>` 安全查询方法，并提供 `LoadAsync(Func<string, Task<byte[]>> readBytesAsync)` 异步加载入口。

默认导出所有 sheet。可以通过 `--sheet` 指定只导出某一个 sheet。

### 配置文件传参

所有子命令都支持通过 `--config` 指定 `key=value` 配置文件。配置文件适合保存批处理、CI 或项目固定导出参数；命令行显式传入的参数优先级更高，可临时覆盖配置文件。

项目根目录提供了 `config.example` 作为完整配置模板。实际使用时可以复制该文件，并按 `convert`、`decode` 或 `codegen` 需要保留对应字段。配置文件每行一个参数，格式为 `key=value`；空行会被忽略，行首为 `#` 的内容表示注释。

参数合并顺序为：

```text
默认值 < 配置文件 < 命令行参数
```

示例 `iotaexcel.convert.conf`：

```ini
# 输入 Excel 文件或目录
input=./excels
output=./out/bytes
convertFormat=bin
decodeFormat=json
recursive=true
sheet=
overwrite=true
target=both
checkRef=true
selfDescribing=false
logLevel=info
logFormat=text
```

示例命令：

```bash
iotaexcel convert --config ./iotaexcel.convert.conf
iotaexcel convert --config ./iotaexcel.convert.conf --output ./out/debug-json --format json
```

配置文件字段使用 camelCase 名称。`convertFormat` 对应 convert 的 `--format`，`decodeFormat` 对应 decode 的 `--format`，二者分离避免混用；`checkRef` 对应 `--check-ref`，`selfDescribing` 对应 `--self-describing`，`schemaInput` 对应 `--schema-input`，`printMode` 对应 `--print-mode`，`logLevel`、`logFormat`、`logFile` 对应日志参数。未知字段会被视为配置错误并停止执行。

### 业务层接入生成代码

业务项目需要同时接入 codegen 输出的 `Excel名.config.cs` 和 `IotaExcelRuntime.cs`，并把 convert 输出的 `.bytes` 文件随项目资源一起分发。运行时读取对应 `.bytes` 字节后，调用生成的 table loader 即可得到按唯一 key 建立索引的数据表。

```csharp
using System;
using System.IO;
using DataConfig;

var itemBytes = File.ReadAllBytes("Config_ItemConfig.bytes");
var itemTable = ItemConfigTable.Load(itemBytes);

if (itemTable.TryGetByid(1001, out var item))
{
    Console.WriteLine(item.name);
}
```

如果业务资源系统是异步接口，可以使用生成的 `LoadAsync`：

```csharp
var itemTable = await ItemConfigTable.LoadAsync(ReadBytesAsync);
```

生成的 C# reader 直接按生成时的 schema 解析 `.bytes`，业务层不需要再调用 CLI 的 `decode` 命令，也不需要在运行时读取 Excel。

## 核心规则

- 每个 sheet 至少需要 5 行。
- 第 1 行：字段名。使用 `*id` 或 `id*` 标记唯一 key。
- 第 2 行：字段类型。
- 第 3 行：字段用途，支持 `client`、`server`、`all`、`comment`。
- 第 4 行：字段注释。
- 第 5 行及之后：数据行。

每个 sheet 导出一个 `.bytes` 文件。`.bytes` 可通过 `decode` 命令反解析为 CSV 或 JSON。每个 Excel 文件导出一个 C# 代码文件。C# 代码默认使用 `DataConfig` 命名空间。

## 常用参数

- `--input`：输入文件或目录。
- `--output`：输出目录。
- `--config`：`key=value` 配置文件路径；配置文件中的参数会覆盖默认值，命令行显式参数会覆盖配置文件。
- `--format`：输出格式，`convert` 支持 `csv`、`json`、`bin`，`decode` 支持 `csv`、`json`。
- `--sheet`：可选，指定 sheet 名称或从 1 开始的 sheet 索引。
- `--target`：导出目标，支持 `client`、`server`、`both`。
- `--check-ref`：校验 `ref<T>` 引用目标表和 key 是否存在。
- `--self-describing`：控制 `.bytes` 是否内嵌字段名和类型名，默认 `true`；设为 `false` 可减小体积。
- `--schema-input`：仅 `decode` 需要，用于解析 `--self-describing=false` 生成的 `.bytes`，指向原 Excel 文件或目录。
- `--print`：仅 `decode` 使用，按顺序打印 `.bytes` 文件头、字段元数据和每行字段值。
- `--print-mode`：仅 `decode --print` 使用，支持 `verbose` 和 `concise`。
- `--log-level`：日志级别，支持 `debug`、`info`、`warn`、`error`。
- `--log-format`：日志格式，支持 `text`、`json`。
- `--log-file`：可选，指定日志文件路径。

## 开发计划

- 支持更多导出语言。当前 codegen 仅支持 C#，后续可能增加 C、C++、Go、Java 等目标语言。
- 扩展 Excel 特性支持。当前读取能力覆盖配置导出所需的基础 XLSX XML、sharedStrings 和 inlineStr，后续可继续补充公式、更多单元格类型和样式相关能力。
- 支持数据加密。为 `.bytes` 产物增加可选加密流程，便于客户端资源分发时保护配置内容。
- 增强导出代码表达能力。后续可支持枚举、结构体等 schema 定义，并在生成代码中输出更贴近业务模型的类型。

## 文档

- `docs/format.md`：Excel、`.bytes` 和 `.iotaignore` 规则。
- `docs/codegen.md`：C# 代码生成规则。
- `docs/logging.md`：日志约定。
- `docs/excel-cli-plan_dab03005.plan.md`：原始项目 MVP plan 归档。
- `docs/contributing.md`：提交格式和验证步骤。
