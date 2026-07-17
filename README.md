# iotaexcel

`iotaexcel` 是一个跨平台命令行工具，用于把规则化 Excel 配置表导出为 CSV、JSON 或 `.bytes` 二进制资源，并生成多语言读取代码。

当前主要能力：

- 读取基础 `.xlsx` 工作簿，支持 `sharedStrings`、`inlineStr` 和稀疏单元格。
- 按 Excel 表头规则校验 schema、唯一 key、字段用途和字段类型。
- 导出 CSV、JSON、`.bytes`；`.bytes` 使用 protobuf 风格 TLV 行数据编码。
- 支持自描述和非自描述 `.bytes`，非自描述文件可通过外部 Excel schema decode。
- 支持 `client`、`server`、`both` 目标过滤字段。
- 支持 `ref<T>` 引用检查。
- 支持生成 C#、Go、C++、Java、JavaScript、Python 和 Swift 读取代码。
- 支持结构化日志、批处理脚本、多平台构建和 GitHub Actions 发布流程。

## 环境要求

- 从源码构建需要 Go 1.19 或更高版本。
- 构建和测试脚本使用本机 Go 工具链的默认全局 build cache，不会覆盖 `GOCACHE`。

## 构建

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

构建产物输出到 `dist/`。默认构建使用 `CGO_ENABLED=0`、`-trimpath`、`-ldflags="-s -w"`，并生成 `dist/sha256sums.txt`。

## 验证

提交前建议运行文档检查和 Go 测试。

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

GitHub Actions 会在 push 和 pull request 时执行格式检查、文档检查、Go 测试，并运行 `scripts/build.sh --all` 验证多平台发布产物。

推送 `v*` tag 时，GitHub Actions 会创建 GitHub Release，并上传 `dist/` 下的多平台可执行文件和 `sha256sums.txt`。

## 快速开始

### 导出

```bash
iotaexcel convert --input ./excels --output ./out --format bin
iotaexcel convert --input ./excels --output ./out --format bin --self-describing=false
iotaexcel convert --input ./excels --output ./out --format json
iotaexcel convert --config ./iotaexcel.convert.conf
```

`convert` 读取 Excel 文件或目录，并按 `--format csv|json|bin` 写出结果。`.bytes` 文件默认包含字段名和类型名；使用 `--self-describing=false` 可减小体积。

### 反解析

```bash
iotaexcel decode --input ./out --output ./decoded --format json
iotaexcel decode --input ./out --schema-input ./excels --output ./decoded --format json --self-describing=false
iotaexcel decode --input ./out --output ./decoded --format json --print
iotaexcel decode --input ./out --output ./decoded --format json --print --print-mode concise
iotaexcel decode --config ./iotaexcel.decode.conf
```

`decode` 读取 `.bytes`，反解析为 CSV 或 JSON。非自描述 `.bytes` 需要通过 `--schema-input` 指向原 Excel schema。

### 生成读取代码

```bash
iotaexcel codegen --input ./excels --output ./generated --lang csharp
iotaexcel codegen --input ./excels --output ./generated --lang go
iotaexcel codegen --input ./excels --output ./generated --lang cpp
iotaexcel codegen --input ./excels --output ./generated --lang java
iotaexcel codegen --input ./excels --output ./generated --lang javascript
iotaexcel codegen --input ./excels --output ./generated --lang python
iotaexcel codegen --input ./excels --output ./generated --lang swift
iotaexcel codegen --config ./iotaexcel.codegen.conf
```

`codegen` 根据 Excel schema 生成读取 `.bytes` 的业务代码和共享 runtime。各语言的输出文件、API、版本要求和兼容风险见 [docs/codegen.md](docs/codegen.md)。

### 配置文件

所有子命令都支持 `--config` 读取 `key=value` 配置文件。配置文件适合保存批处理、CI 或项目固定导出参数；命令行显式参数优先级更高。

项目根目录提供 [config.example](config.example) 作为完整配置模板。配置字段使用 camelCase 名称，例如：

- `convertFormat` 对应 convert 的 `--format`
- `decodeFormat` 对应 decode 的 `--format`
- `checkRef` 对应 `--check-ref`
- `selfDescribing` 对应 `--self-describing`
- `schemaInput` 对应 `--schema-input`
- `printMode` 对应 `--print-mode`
- `logLevel`、`logFormat`、`logFile` 对应日志参数

## 规则概览

- 每个 sheet 至少需要 5 行：字段名、字段类型、字段用途、字段注释、数据行。
- 字段名可用 `*id` 或 `id*` 标记唯一 key。
- 字段名、sheet 名和 Excel 文件名必须满足 `^[A-Za-z_][A-Za-z0-9_]*$`。
- 字段用途支持 `client`、`server`、`all`、`comment` 及常用别名。
- 每个 sheet 导出一个 `.bytes` 文件，命名为 `Excel名_Sheet名Config.bytes`。

完整 Excel 规则、`.bytes` 格式、decode 输出和 `.iotaignore` 规则见 [docs/format.md](docs/format.md)。

## 技术取舍

本项目使用 Go 实现 CLI，以获得简单的跨平台构建、单文件可执行产物和较少运行时依赖。当前项目没有引入第三方 Go module；XLSX 读取、`.iotaignore` 解析、TLV 编解码和多语言 codegen 都由项目内代码实现。

对应取舍是：`.bytes` 是项目自定义格式，生态和 schema 演进能力弱于 protobuf 等成熟标准，但更贴近“Excel 配置表 -> 二进制资源 -> 业务读取代码”的工作流。

## 文档

- [docs/format.md](docs/format.md)：Excel、`.bytes`、decode 和 `.iotaignore` 规则。
- [docs/codegen.md](docs/codegen.md)：多语言代码生成规则、版本要求、接入示例和类型映射。
- [docs/logging.md](docs/logging.md)：日志参数和结构化字段。
- [docs/contributing.md](docs/contributing.md)：提交格式、文档同步和验证步骤。
- [tests/README.md](tests/README.md)：测试 fixture、生成脚本和测试输出目录。
- [docs/excel-cli-plan_dab03005.plan.md](docs/excel-cli-plan_dab03005.plan.md)：早期 MVP plan 归档，仅保留历史设计上下文。
