# 日志

`iotaexcel` 使用结构化日志输出，便于本地排查、脚本收集和 CI 分析。

## 参数

- `--log-level`：日志级别，支持 `debug`、`info`、`warn`、`error`。
- `--log-format`：日志格式，支持 `text` 和 `json`。
- `--log-file`：可选日志文件输出路径。

默认日志会输出到 stderr，避免污染命令生成的输出文件。

## 字段

常见结构化字段：

- `command`
- `source`
- `sheet`
- `format`
- `target`
- `output`
- `field`
- `row`
- `column`
- `error`
- `duration_ms`

最终的 `summary` 事件会包含成功、失败、跳过、类型转换错误、默认值使用、ref 校验错误以及输出文件数量等统计信息。
