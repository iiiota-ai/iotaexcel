# 贡献指南

## Commit Format

提交信息使用简化版 Conventional Commits 格式：

```text
<type>(<scope>): <summary>
```

推荐的 `type`：

- `feat`
- `fix`
- `docs`
- `test`
- `refactor`
- `build`
- `ci`
- `chore`

推荐的 `scope`：

- `cli`
- `xlsx`
- `schema`
- `binary`
- `codegen`
- `logging`
- `docs`
- `ignore`
- `release`

示例：

```text
feat(schema): 支持字段名星号标记唯一 key
fix(binary): 修正 sint64 ZigZag 编码
docs(format): 补充 ref<Item> 单元格示例
```

## 文档同步

每次修改用户可见能力时，都需要在同一次变更中同步更新 `README.md` 或 `docs/`。如果某次代码变更不需要更新文档，需要在 PR 或提交说明中解释原因。

## 测试目录

单元测试应放在被测试 Go 包的同级目录中。

跨包集成测试共享的 fixture 放在 `tests/testdata/` 下：

- `tests/testdata/excels/`：源 `.xlsx` fixture。
- `tests/testdata/expected/csv/`：预期 CSV 输出。
- `tests/testdata/expected/json/`：预期 JSON 输出。
- `tests/testdata/expected/bytes/`：预期 `.bytes` 输出或 decode 快照。
- `tests/testdata/expected/csharp/`：预期生成的 C# 文件。
- `tests/testdata/expected/go/`：预期生成的 Go 文件。
- `tests/testdata/expected/cpp/`：预期生成的 C++ 文件。

## 验证

提交前需要运行文档检查和 Go 测试：

```bash
scripts/check-docs.sh
scripts/test.sh
```

Windows 环境：

```powershell
scripts/check-docs.ps1
scripts/test.ps1
```
