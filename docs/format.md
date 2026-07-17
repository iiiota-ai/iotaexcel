# 格式规范

## Excel 表头

每个 sheet 至少需要包含 5 行：

1. 字段名
2. 字段类型
3. 字段用途
4. 字段注释
5. 数据行

第 1 行字段名可以使用后缀标记约束。去掉标记后，字段名必须匹配 `^[A-Za-z_][A-Za-z0-9_]*$`。

- `#`：key 字段，隐含 `*` 和 `!`，即必填且不可重复。示例：`id#`。
- `*`：必填字段，数据行中单元格不可为空。示例：`name*`。
- `!`：唯一字段，非空值不可重复。`!` 默认不表示必填，空值不参与唯一校验；必填唯一可组合为 `email*!`。

标记只能放在字段名末尾，同一类标记不能重复。每个 sheet 必须且只能有一个 key 字段。

生成读取代码时，非 key 的 `!` 字段会生成额外查询索引。为了避免跨语言索引语义不一致，`!` 字段当前限制为 `int`、`int32`、`int64` 或 `string`；非必填的 `!` 字段仅支持 `string`，因为数字空值导出后无法和真实默认值区分。

## 类型

- `bool`：支持 `true`、`false`、`1` 或 `0`。
- `int`、`int32`、`int64`：有符号整数，使用 ZigZag + varint 编码。
- `float`、`double`：十进制文本。
- `string`：文本，默认值为 `""`。
- `bytes`：按 UTF-8 文本 payload 写入，生成代码按目标语言的字节容器读取。
- `datetime`：格式为 `YYYY-MM-DD HH:mm:ss`，默认值为公元 1 年对应的秒级时间戳。
- `array<T>`：示例 `a|b|c`。
- `map<K,V>`：示例 `a:1|b:2`。
- `ref<Item>`：引用 `Item` sheet 的 key，例如 `1001`。

## 字段用途

字段用途大小写不敏感，并支持别名：

- client：`client`、`c`、`cli`、`clientonly`
- server：`server`、`s`、`srv`、`serveronly`
- all：`all`、`cs`、`common`、`shared`
- comment：`comment`、`note`、`remark`、`ignore`、`skip`

`comment` 字段不会写入 `.bytes`。

## .bytes

每个 sheet 会导出一个 `.bytes` 文件，文件名格式为 `Excel名_Sheet名Config.bytes`。payload 使用 protobuf 风格 TLV 格式：

```text
tag = (fieldNo << 3) | wireType
```

`fieldNo` 是从 1 开始的 Excel 列号。已经发布的表不应在已有二进制字段中间插入新字段。

外层文件依次包含：

- magic：`IOTB`
- 二进制版本号 binaryVersion
- schemaHash
- key 字段编号
- 自描述标记
- 字段元数据
- 行 payload

`schemaHash` 是规范化 schema 内容的 SHA-256 摘要。参与计算的内容包括 sheet、target、binaryVersion、key、字段名、字段类型、usage、fieldNo、wireType 和是否写入二进制的标记。

使用 `convert --format bin --self-describing=true` 时，字段元数据包含 `fieldNo`、字段名、字段类型和 `fieldFlags`。此时 `.bytes` 文件可以独立 decode。`fieldFlags` 是 bitmask：`1` 表示 key，`2` 表示 required，`4` 表示 unique。

使用 `convert --format bin --self-describing=false` 时，字段元数据只包含 `fieldNo`。这会减小文件体积，但 `decode` 时必须提供原始 Excel schema：

```bash
iotaexcel decode --input ./out --schema-input ./excels --output ./decoded --format json --self-describing=false
```

解码器会通过 `schemaHash` 匹配外部 schema，而不是依赖文件名匹配。

字段值按 wire type 编码：

- varint：`bool`、`int`、`int32`、`int64`、`datetime`；有符号整数使用 ZigZag。
- fixed32：`float`，小端 IEEE 754。
- fixed64：`double`，小端 IEEE 754。
- length-delimited：`string`、`bytes`、`array<T>`、`map<K,V>`、`ref<T>`。

当前嵌套值按 UTF-8 文本 payload 存储：`array<T>` 使用 `|` 分隔，`map<K,V>` 使用 `key:value|key:value` 形式，并按 key 排序以保证输出稳定。

## Decode 输出

`.bytes` 文件可以反解析为 CSV 或 JSON：

```bash
iotaexcel decode --input ./out --output ./decoded --format json
iotaexcel decode --input ./out --output ./decoded --format csv
iotaexcel decode --input ./out --output ./decoded --format json --print
iotaexcel decode --input ./out --output ./decoded --format json --print --print-mode concise
```

对于自描述文件，`decode` 命令会从 `.bytes` 头部读取字段元数据。对于非自描述文件，它会从 `.bytes` 读取 `fieldNo`，再通过 `--schema-input` 解析字段名和字段类型。输出会保留字段顺序，并为每个输入 `.bytes` 文件写出一个结果文件。

CSV 预览和 decode CSV 会在表头中恢复字段约束标记：key 字段显示为 `id#`，非 key 的唯一字段显示为 `name!`，必填字段显示为 `label*`，必填唯一字段显示为 `email!*`。

使用 `--print` 时，decode 还会按读取顺序把可读 trace 输出到 stdout：

- 文件和头部元数据：source、version、selfDescribing、schemaHash、keyFieldNo、fieldCount、rowCount
- 字段元数据：字段顺序、fieldNo、字段名、类型、fieldFlags、key、required、unique
- 行数据：每行以及按字段元数据顺序排列的字段值

`--print-mode concise` 只打印字面量，不打印说明标签。输出顺序为：

- `relPath`、`sourcePath`、`IOTB`、`version`、`selfDescribing`、`schemaHash`、`keyFieldNo`、`fieldCount`
- 每个字段一行，使用 tab 分隔：`fieldNo`、`name`、`type`、`fieldFlags`
- `rowCount`
- 每个单元格一行，使用 tab 分隔：`tag`、`fieldNo`、`wireType`、`value`

## XLSX 支持

读取器支持 `sharedStrings` 和稀疏单元格。存在 `inlineStr` 时也会处理。当前不支持富文本格式、样式、公式、图表和宏。

## .iotaignore

`.iotaignore` 使用轻量级 `.gitignore` 风格子集：

- 使用 `#` 书写注释
- 目录忽略，例如 `temp/`
- 文件名模式，例如 `*.bak.xlsx`
- 路径模式，例如 `legacy/*.xlsx`

默认会跳过类似 `~$*.xlsx` 的 Excel 临时文件。
