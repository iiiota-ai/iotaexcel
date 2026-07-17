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
feat(schema): 支持字段约束标记
fix(binary): 修正 sint64 ZigZag 编码
docs(format): 补充 ref<Item> 单元格示例
```

## 文档同步

每次修改用户可见能力时，都需要在同一次变更中同步更新 `README.md` 或 `docs/`。如果某次代码变更不需要更新文档，需要在 PR 或提交说明中解释原因。

## 测试

单元测试应放在被测试 Go 包的同级目录中。跨包集成测试、fixture 目录结构和 fixture 生成脚本见 `tests/README.md`。

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
