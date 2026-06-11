# Contributing to Golang Best Practices

感谢你对本项目的关注！我们欢迎所有形式的贡献。

## 如何贡献

### 报告问题

如果你发现了文档中的错误或有改进建议：

1. 在 [Issues](https://github.com/difyz9/golang-best-practices/issues) 页面搜索是否已有相关问题
2. 如果没有，创建一个新的 Issue，提供以下信息：
   - 清晰的问题描述
   - 相关的文档位置
   - 建议的改进方案（可选）

### 提交改进

1. **Fork 项目**
   ```bash
   git clone https://github.com/difyz9/golang-best-practices.git
   cd golang-best-practices
   ```

2. **创建分支**
   ```bash
   git checkout -b feature/your-feature-name
   # 或
   git checkout -b fix/your-bug-fix
   ```

3. **进行修改**
   - 保持文档格式一致
   - 确保代码示例可运行
   - 添加必要的注释

4. **提交更改**
   ```bash
   git add .
   git commit -m "描述你的更改"
   ```

5. **推送到 GitHub**
   ```bash
   git push origin feature/your-feature-name
   ```

6. **创建 Pull Request**
   - 提供清晰的 PR 标题和描述
   - 关联相关的 Issue（如果有）
   - 等待审核和反馈

## 文档规范

### Markdown 格式

- 使用 4 空格缩进
- 代码块使用三个反引号，并指定语言
- 使用清晰的标题层级（# ## ### ####）
- 表格对齐要整齐

### 代码示例

```go
// ✅ 推荐：包含完整的 package 和 import
package handler

import (
    "github.com/gin-gonic/gin"
)

func Example() {
    // 代码示例
}
```

```go
// ❌ 不推荐：缺少上下文的代码片段
func Example() {
    // ...
}
```

### 注释规范

- 中文文档使用中文注释
- 英文文档使用英文注释
- 保持注释简洁明了

## 提交信息规范

使用语义化的提交信息：

- `feat: 添加新功能` - 新功能
- `fix: 修复问题` - Bug 修复
- `docs: 更新文档` - 文档更新
- `style: 格式调整` - 不影响代码运行的格式修改
- `refactor: 重构代码` - 代码重构
- `test: 测试相关` - 添加或修改测试
- `chore: 其他修改` - 构建过程或辅助工具的变动

示例：
```bash
git commit -m "docs: 更新 FX 依赖注入示例代码"
git commit -m "feat: 添加 GraphQL 集成指南"
git commit -m "fix: 修正 SKILL.md 中的错误示例"
```

## 审核流程

1. 提交 PR 后，维护者会进行审核
2. 如有需要修改的地方，会在 PR 中提出
3. 修改完成后，会合并到主分支
4. 合并后会自动更新 CHANGELOG.md

## 行为准则

- 尊重所有贡献者
- 保持友好和建设性的讨论
- 接受建设性的批评
- 关注对项目最有利的事情

## 问题咨询

如有任何问题，可以通过以下方式联系：

- GitHub Issues: 技术问题和功能建议
- GitHub Discussions: 一般性讨论和问答

---

再次感谢你的贡献！🎉
