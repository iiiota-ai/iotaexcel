# 代码生成

MVP 阶段代码生成目标语言为 C#。

```bash
iotaexcel codegen --input ./excels --output ./generated --lang csharp
iotaexcel codegen --input ./excels --output ./generated --lang go
iotaexcel codegen --input ./excels --output ./generated --lang cpp
iotaexcel codegen --input ./excels --output ./generated --lang java
iotaexcel codegen --input ./excels --output ./generated --lang javascript
iotaexcel codegen --input ./excels --output ./generated --lang python
iotaexcel codegen --input ./excels --output ./generated --lang swift
```

## 命名规则

- Excel 文件名会追加 `.config` 后缀作为生成的代码文件名。例如 `Config.xlsx` 会生成 `Config.config.cs`。
- Sheet 名会追加 `Config` 后缀作为 C# 数据类名。例如 `Item` 会生成 `ItemConfig`。
- 每个 sheet 还会生成一个一一对应的 loader/table 类，命名为 `Sheet名ConfigTable`。例如 `Item` 会生成 `ItemConfigTable`。
- table 类内部数据集合字段名固定为 `datas`，只读公开属性为 `Datas`。
- table 类只生成安全查询方法：`TryGetBy<Key字段名>`。`<Key字段名>` 直接使用 sheet 中的 key 字段名，不做首字母大小写转换。例如 key 字段为 `id` 时生成 `TryGetByid`。
- table 类同时生成异步加载方法 `LoadAsync(Func<string, Task<byte[]>> readBytesAsync)`。异步方法只负责读取 `.bytes`，读取完成后复用同步 `Load(byte[])` 解码；读取文件名遵循 `Excel名_Sheet名Config.bytes`。
- 两者都必须匹配 `^[A-Za-z_][A-Za-z0-9_]*$`。
- C# 命名空间默认是 `DataConfig`。

## CodegenSchema

生成器使用规范化后的 schema 数据：

- 源 Excel 文件
- sheet 名
- target
- key 字段
- 字段列表
- fieldNo
- wireType
- binaryVersion
- schemaHash

生成的读取代码必须使用和 `.bytes` 写入端一致的 `fieldNo` 和 `wireType`。

## 输出文件

每次 codegen 会输出两类 C# 文件：

- `<Excel文件名>.config.cs`：包含该 Excel 中各个 sheet 对应的业务配置数据类和 table loader 类。
- `IotaExcelRuntime.cs`：共享 `.bytes` 读取 runtime，包含 `IotaBytesReader` 和 `IotaBytesRuntime`。

批量处理多个 Excel 时，`IotaExcelRuntime.cs` 只需要一份。生成器会复用内容一致的 runtime 文件，避免每个业务配置文件重复内嵌相同读取逻辑。

## 读取代码约定

生成的 C# table loader 类会暴露基于 key 查询的 API：

```csharp
SheetNameConfigTable Load(byte[] data)
Task<SheetNameConfigTable> LoadAsync(Func<string, Task<byte[]>> readBytesAsync)
```

生成的 reader 会解析 `.bytes` 外层头部，并要求文件版本等于当前 binaryVersion；随后读取 `selfDescribing` 标记并据此跳过字段元数据，再按 protobuf 风格 TLV 解码每一行，最终返回以 sheet 唯一 key 建立索引的字典。

## 业务层接入方式

业务项目接入导出结果时，需要使用同一批 Excel 生成出来的两类产物：

- `.bytes` 文件：由 `convert --format bin` 输出，作为业务运行时资源分发。
- C# 代码文件：由 `codegen --lang csharp` 输出，包括 `Excel名.config.cs` 和共享的 `IotaExcelRuntime.cs`。

接入步骤：

1. 把 `Excel名.config.cs` 和 `IotaExcelRuntime.cs` 加入业务 C# 工程编译。
2. 把对应 `.bytes` 文件放入业务项目的资源目录、StreamingAssets、Addressables、AssetBundle、服务器下发目录或其他资源系统中。
3. 运行时读取 `.bytes` 的完整字节数组。
4. 调用对应的 `Sheet名ConfigTable.Load(byte[])` 或 `LoadAsync(Func<string, Task<byte[]>>)` 得到 table 实例。
5. 通过 `TryGetBy<Key字段名>` 按唯一 key 获取单行配置，或通过 `Datas` 遍历全部配置。

同步读取示例：

```csharp
using System;
using System.IO;
using DataConfig;

public static class ConfigBootstrap
{
    public static ItemConfigTable LoadItems(string configDir)
    {
        var path = Path.Combine(configDir, "Config_ItemConfig.bytes");
        var bytes = File.ReadAllBytes(path);
        return ItemConfigTable.Load(bytes);
    }
}

var itemTable = ConfigBootstrap.LoadItems("Configs");
if (itemTable.TryGetByid(1001, out var item))
{
    Console.WriteLine(item.name);
}
```

异步资源系统接入示例：

```csharp
using System;
using System.Threading.Tasks;
using DataConfig;

public static class ConfigBootstrap
{
    public static Task<ItemConfigTable> LoadItemsAsync(Func<string, Task<byte[]>> readBytesAsync)
    {
        return ItemConfigTable.LoadAsync(readBytesAsync);
    }
}

var itemTable = await ConfigBootstrap.LoadItemsAsync(ReadBytesAsync);
```

`LoadAsync` 内部会按生成时约定的文件名调用 `readBytesAsync`，例如 `ItemConfigTable.LoadAsync` 会请求 `Config_ItemConfig.bytes`。业务层只需要在 `readBytesAsync` 中根据文件名从自己的资源系统返回对应字节。

生成的 C# reader 使用编译进代码里的 schema 解析行数据，因此业务运行时不需要调用 CLI 的 `decode` 命令，也不需要读取原始 Excel。`decode` 命令主要用于工具侧排查、导出 CSV/JSON 或验证 `.bytes` 内容。

需要注意：

- `.bytes` 文件、`Excel名.config.cs` 和 `IotaExcelRuntime.cs` 应来自同一次或同一版本的导出流程，避免字段顺序、fieldNo 或类型定义不一致。
- 生成 reader 会严格检查 `.bytes` 格式版本；版本不匹配会直接抛出异常。
- 自描述和非自描述 `.bytes` 都可以被生成 reader 读取。生成 reader 依赖的是代码中的 schema，不依赖 `.bytes` 内嵌字段名和类型名。
- `TryGetBy<Key字段名>` 是安全查询接口；未找到 key 时返回 `false`，业务层应自行决定默认配置、报错或降级策略。

当前 C# reader 支持以下 MVP 类型映射：

- `bool`, `int`, `int32`, `int64`, `datetime`
- `float`, `double`
- `string`, `bytes`
- `array<T>` 映射为 `List<string>?`
- `map<K,V>` 映射为 `Dictionary<string, string>?`
- `ref<T>` 映射为 `string?`

当前仍处于开发阶段，生成的 reader 不做历史版本兼容；binaryVersion 不匹配时会直接拒绝读取。schemaHash 当前主要用于非自描述 `.bytes` 的外部 schema 匹配。
## Go codegen

`codegen --lang go` generates one `<ExcelName>.config.go` file per workbook and one shared `iotaexcel_runtime.go` file. The default Go package is `dataconfig`; use `--package` to override it.

Generated Go APIs follow the same table/key model as C#:

```go
table, err := LoadItemConfigTable(data)
item, ok := table.TryGetByid(1001)
```

For resource systems that load by file name:

```go
table, err := LoadItemConfigTableFrom(readBytes)
```

`LoadItemConfigTableFrom` asks `readBytes` for the generated `.bytes` file name, for example `Config_ItemConfig.bytes`.

Go type mapping:

- `bool` -> `bool`
- `int`, `int32` -> `int32`
- `int64`, `datetime` -> `int64`
- `float` -> `float32`
- `double` -> `float64`
- `string`, `ref<T>` -> `string`
- `bytes` -> `[]byte`
- `array<T>` -> `[]string`
- `map<K,V>` -> `map[string]string`

## C++ codegen

`codegen --lang cpp` generates one `<ExcelName>.config.hpp` file per workbook and one shared `iotaexcel_runtime.hpp` file. The default C++ namespace is `DataConfig`; use `--package` to override it.

Generated C++ APIs follow the same table/key model:

```cpp
auto table = DataConfig::ItemConfigTable::Load(data);
const DataConfig::ItemConfig* item = nullptr;
if (table.TryGetByid(1001, item)) {
    // use item
}
```

For resource systems that load by file name:

```cpp
auto table = DataConfig::ItemConfigTable::LoadFrom(readBytes);
```

`LoadFrom` asks `readBytes` for the generated `.bytes` file name, for example `Config_ItemConfig.bytes`.

C++ type mapping:

- `bool` -> `bool`
- `int`, `int32` -> `std::int32_t`
- `int64`, `datetime` -> `std::int64_t`
- `float` -> `float`
- `double` -> `double`
- `string`, `ref<T>` -> `std::string`
- `bytes` -> `std::vector<std::uint8_t>`
- `array<T>` -> `std::vector<std::string>`
- `map<K,V>` -> `std::unordered_map<std::string, std::string>`

## Java codegen

`codegen --lang java` generates one `<ExcelName>.java` file per workbook and one shared `IotaExcelRuntime.java` file. The default Java package is `dataconfig`; use `--package` to override it.

The workbook file contains one public workbook class, for example `Config`, and each sheet is generated as static nested config/table classes:

```java
Config.ItemConfigTable table = Config.ItemConfigTable.load(data);
Config.ItemConfig item = table.tryGetByid(1001);
```

For resource systems that load by file name:

```java
Config.ItemConfigTable table = Config.ItemConfigTable.loadFrom(readBytes);
```

`loadFrom` asks `readBytes` for the generated `.bytes` file name, for example `Config_ItemConfig.bytes`.

Java type mapping:

- `bool` -> `boolean`
- `int`, `int32` -> `int`
- `int64`, `datetime` -> `long`
- `float` -> `float`
- `double` -> `double`
- `string`, `ref<T>` -> `String`
- `bytes` -> `byte[]`
- `array<T>` -> `List<String>`
- `map<K,V>` -> `Map<String, String>`

## JavaScript codegen

`codegen --lang javascript` generates one `<ExcelName>.config.js` file per workbook and one shared `iotaexcel_runtime.js` file. The output is standard ES Module code and depends only on JavaScript standard APIs such as `Uint8Array`, `DataView`, `TextDecoder`, `Map`, and `BigInt`.

Generated JavaScript APIs follow the same table/key model:

```js
const table = loadItemConfigTable(data);
const item = table.tryGetByid(1001);
```

For asynchronous resource systems that load by file name:

```js
const table = await loadItemConfigTableFrom(readBytes);
```

`loadItemConfigTableFrom` asks `readBytes` for the generated `.bytes` file name, for example `Config_ItemConfig.bytes`. `readBytes` must return a `Uint8Array`, so callers can adapt browser `fetch`, Node `fs/promises`, Electron, or engine-specific asset systems without changing generated code.

JavaScript type mapping:

- `bool` -> `boolean`
- `int`, `int32` -> `number`
- `int64`, `datetime` -> `bigint`
- `float`, `double` -> `number`
- `string`, `ref<T>` -> `string`
- `bytes` -> `Uint8Array`
- `array<T>` -> `string[]`
- `map<K,V>` -> `Map<string, string>`

## Python codegen

`codegen --lang python` generates one `<ExcelName>_config.py` file per workbook and one shared `iotaexcel_runtime.py` file. The output uses only Python standard library modules.

Generated Python APIs follow the same table/key model:

```python
table = load_item_config_table(data)
item = table.try_get_by_id(1001)
```

For resource systems that load by file name:

```python
table = load_item_config_table_from(read_bytes)
```

`load_item_config_table_from` asks `read_bytes` for the generated `.bytes` file name, for example `Config_ItemConfig.bytes`. `read_bytes` may return `bytes`, `bytearray`, or `memoryview`.

Python type mapping:

- `bool` -> `bool`
- `int`, `int32`, `int64`, `datetime` -> `int`
- `float`, `double` -> `float`
- `string`, `ref<T>` -> `str`
- `bytes` -> `bytes`
- `array<T>` -> `list[str]`
- `map<K,V>` -> `dict[str, str]`

## Swift codegen

`codegen --lang swift` generates one `<ExcelName>.config.swift` file per workbook and one shared `IotaExcelRuntime.swift` file. The output uses Foundation `Data` and does not depend on UI frameworks.

Generated Swift APIs follow the same table/key model:

```swift
let table = try loadItemConfigTable(data)
let item = table.tryGetByid(1001)
```

For resource systems that load by file name:

```swift
let table = try ItemConfigTable.loadFrom(readBytes)
```

`loadFrom` asks `readBytes` for the generated `.bytes` file name, for example `Config_ItemConfig.bytes`.

Swift type mapping:

- `bool` -> `Bool`
- `int`, `int32` -> `Int32`
- `int64`, `datetime` -> `Int64`
- `float` -> `Float`
- `double` -> `Double`
- `string`, `ref<T>` -> `String`
- `bytes` -> `Data`
- `array<T>` -> `[String]`
- `map<K,V>` -> `[String: String]`
