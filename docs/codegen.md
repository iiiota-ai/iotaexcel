# 代码生成

`codegen` 根据 Excel schema 生成读取 `.bytes` 的业务代码。每个 workbook 会生成一个业务配置文件，并在输出根目录生成一个共享 runtime 文件。

```bash
iotaexcel codegen --input ./excels --output ./generated --lang csharp
iotaexcel codegen --input ./excels --output ./generated --lang go
iotaexcel codegen --input ./excels --output ./generated --lang cpp
iotaexcel codegen --input ./excels --output ./generated --lang java
iotaexcel codegen --input ./excels --output ./generated --lang javascript
iotaexcel codegen --input ./excels --output ./generated --lang python
iotaexcel codegen --input ./excels --output ./generated --lang swift
```

## 输出文件

| 目标语言 | 业务文件 | runtime 文件 | 默认包/命名空间 |
| --- | --- | --- | --- |
| C# | `<ExcelName>.config.cs` | `IotaExcelRuntime.cs` | `DataConfig` |
| Go | `<ExcelName>.config.go` | `iotaexcel_runtime.go` | `dataconfig` |
| C++ | `<ExcelName>.config.hpp` | `iotaexcel_runtime.hpp` | `DataConfig` |
| Java | `<ExcelName>.java` | `IotaExcelRuntime.java` | `dataconfig` |
| JavaScript | `<ExcelName>.config.js` | `iotaexcel_runtime.js` | 不使用 |
| Python | `<ExcelName>_config.py` | `iotaexcel_runtime.py` | 不使用 |
| Swift | `<ExcelName>.config.swift` | `IotaExcelRuntime.swift` | 不使用 |

`--package` 用于覆盖 C# 命名空间、Go/Java 包名或 C++ 命名空间。JavaScript、Python 和 Swift 当前忽略该选项。

## 生成模型

每个 sheet 会生成：

- `SheetNameConfig`：单行配置数据类型。
- `SheetNameConfigTable`：按 key 和非 key 唯一字段建索引的 table/loader 类型。
- 安全查询方法：按 key 或 `!` 唯一字段返回单行配置，未命中时返回目标语言对应的空值/`false`。
- 直接加载入口：从完整 `.bytes` 字节加载 table。
- 文件名回调加载入口：把约定的 `.bytes` 文件名交给业务层读取，再复用直接加载入口解码。

生成代码直接按编译进代码里的 schema 解析 `.bytes`，业务运行时不需要读取 Excel，也不需要调用 CLI 的 `decode` 命令。

## 版本要求与兼容风险

下表是当前生成代码的建议最低版本。这里按生成代码实际使用的语言特性和标准库 API 估算，业务项目还需要结合自己的构建系统、运行平台和引擎环境验证。

| 目标语言 | 建议最低版本 | 主要依据 |
| --- | --- | --- |
| C# | C# 8.0 / .NET Standard 2.1 或 .NET Core 3.x+ | 使用 nullable reference type 写法，例如 `string?`、`List<string>?`，并提供 `Task` 异步加载入口。 |
| Go | Go 1.19 | 项目 `go.mod` 使用 `go 1.19`；生成代码本身不依赖泛型，但建议与工具链保持一致。 |
| C++ | C++11 | 使用 `auto`、`nullptr`、`std::move`、`std::function`、`std::unordered_map`、`std::uint8_t` 等 C++11 能力。 |
| Java | Java 8 | 使用 `@FunctionalInterface`，`loadFrom` 适合用 Java 8 lambda 接入资源读取。 |
| JavaScript | ES2020 | 使用 ES Module、`class`、`async/await`、`BigInt`、`Uint8Array`、`DataView`、`TextDecoder`。 |
| Python | Python 3.10 | 使用 `bytes | bytearray | memoryview` 联合类型语法，以及 `list[str]`、`dict[str, str]` 泛型内置类型标注。 |
| Swift | Swift 5 | 使用 `Foundation.Data`、`throws`、泛型数组/字典和 `String(decoding:as:)` 等现代 Swift 写法。 |

主要兼容风险：

- JavaScript 的 `int64` 和 `datetime` 会映射为 `bigint`，可以避免 64 位整数精度丢失，但旧浏览器、旧 WebView、部分小游戏或嵌入式 JS 引擎可能不支持 `BigInt`。
- Python 当前类型标注要求 Python 3.10。若业务项目仍在 Python 3.8/3.9，需要把联合类型语法改成 `typing.Union[...]`，或在生成器中增加兼容模式。
- C# 的 nullable reference type 写法依赖较新的 C# 编译器配置。老 Unity、老 .NET Framework 或关闭 nullable 支持的项目可能需要降级生成模板。
- Swift 当前只做生成内容检查，未在所有目标平台做编译验证；iOS、macOS、Linux Swift 工具链和 Foundation 可用性仍建议由业务项目自行验证。
- 各语言的回调加载入口只负责把 `.bytes` 文件名交给业务层。文件系统、网络、包体资源、Addressables、AssetBundle、WebView 沙盒等平台差异需要业务层适配。

## CodegenSchema

生成器复用 schema 校验后的 workbook 数据。生成代码和 `.bytes` 写入端必须使用同一套字段编号和 wire type 规则。

关键数据包括：

- 源 Excel 文件相对路径
- sheet 名
- target
- key 字段
- 字段列表
- `fieldNo`
- `wireType`
- `binaryVersion`
- `schemaHash`

`fieldNo` 来自从 1 开始的 Excel 列号。生成 reader 会按 `fieldNo` 解析 TLV 行数据，未知字段会按 `wireType` 跳过。

## 类型映射

| Excel 类型 | C# | Go | C++ | Java | JavaScript | Python | Swift |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bool` | `bool` | `bool` | `bool` | `boolean` | `boolean` | `bool` | `Bool` |
| `int`, `int32` | `int` | `int32` | `std::int32_t` | `int` | `number` | `int` | `Int32` |
| `int64`, `datetime` | `long` | `int64` | `std::int64_t` | `long` | `bigint` | `int` | `Int64` |
| `float` | `float` | `float32` | `float` | `float` | `number` | `float` | `Float` |
| `double` | `double` | `float64` | `double` | `double` | `number` | `float` | `Double` |
| `string`, `ref<T>` | `string` / `string?` | `string` | `std::string` | `String` | `string` | `str` | `String` |
| `bytes` | `byte[]` | `[]byte` | `std::vector<std::uint8_t>` | `byte[]` | `Uint8Array` | `bytes` | `Data` |
| `array<T>` | `List<string>?` | `[]string` | `std::vector<std::string>` | `List<String>` | `string[]` | `list[str]` | `[String]` |
| `map<K,V>` | `Dictionary<string, string>?` | `map[string]string` | `std::unordered_map<std::string, std::string>` | `Map<String, String>` | `Map<string, string>` | `dict[str, str]` | `[String: String]` |

当前 `array<T>` 和 `map<K,V>` 在 `.bytes` 中仍以 UTF-8 文本 payload 保存，生成代码会还原为各语言的字符串列表或字符串字典。

## 接入示例

C#:

```csharp
using DataConfig;

var itemBytes = File.ReadAllBytes("Config_ItemConfig.bytes");
var itemTable = ItemConfigTable.Load(itemBytes);
if (itemTable.TryGetByid(1001, out var item))
{
    Console.WriteLine(item.name);
}

var itemTableFromAssets = await ItemConfigTable.LoadAsync(ReadBytesAsync);
```

Go:

```go
itemBytes, err := os.ReadFile("Config_ItemConfig.bytes")
if err != nil {
    return err
}
itemTable, err := dataconfig.LoadItemConfigTable(itemBytes)
if err != nil {
    return err
}
item, ok := itemTable.TryGetByid(1001)

itemTableFromAssets, err := dataconfig.LoadItemConfigTableFrom(readBytes)
```

C++:

```cpp
auto itemTable = DataConfig::ItemConfigTable::Load(ReadAllBytes("Config_ItemConfig.bytes"));
const DataConfig::ItemConfig* item = nullptr;
if (itemTable.TryGetByid(1001, item)) {
    // use item
}

auto itemTableFromAssets = DataConfig::ItemConfigTable::LoadFrom(readBytes);
```

Java:

```java
Config.ItemConfigTable table = Config.ItemConfigTable.load(data);
Config.ItemConfig item = table.tryGetByid(1001);

Config.ItemConfigTable tableFromAssets = Config.ItemConfigTable.loadFrom(readBytes);
```

JavaScript:

```js
import { ItemConfigTable, loadItemConfigTableFrom } from "./generated/Config.config.js";

const table = ItemConfigTable.load(bytes);
const item = table.tryGetByid(1001);

const tableFromAssets = await loadItemConfigTableFrom(readBytes);
```

Python:

```python
from Config_config import ItemConfigTable, load_item_config_table_from

table = ItemConfigTable.load(item_bytes)
item = table.try_get_by_id(1001)

table_from_assets = load_item_config_table_from(read_bytes)
```

Swift:

```swift
let table = try ItemConfigTable.load(data)
let item = table.tryGetByid(1001)

let tableFromAssets = try ItemConfigTable.loadFrom(readBytes)
```

## 版本与 schema 兼容

生成 reader 会检查 `.bytes` 文件中的 `binaryVersion`。版本不匹配时会拒绝读取。

自描述和非自描述 `.bytes` 都可以被生成 reader 读取。生成 reader 依赖代码中的 schema，不依赖 `.bytes` 内嵌字段名和类型名；因此 `.bytes` 文件和生成代码应来自同一次导出流程，或至少来自兼容的 Excel schema。

已经发布并被业务读取的表，不建议在已有二进制字段中间插入新字段。新增字段更适合追加到末尾，以降低旧代码读取新文件时的兼容风险。
