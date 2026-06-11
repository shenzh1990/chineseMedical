# Golang Best Practices & FX Skills 📚

<div align="center">

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)
![Last Updated](https://img.shields.io/badge/last%20updated-2026--02--06-green)

**A comprehensive collection of Golang best practices and Uber FX dependency injection skills**

[English](#english) | [中文](#中文)

</div>

---

## 中文

### 📖 简介

本技能包提供了一套完整的 Golang 开发最佳实践和 Uber FX 依赖注入框架的实战指南，适合从入门到高级的各个层次开发者。

### 🎯 包含内容

#### 核心技能文档

| 文档 | 说明 | 难度 | 适用场景 |
|------|------|------|---------|
| **[SKILL.md](./SKILL.md)** | Golang 最佳实践完整指南 | ⭐⭐⭐ | 所有 Go 项目 |
| **[fx-basic-integration.md](./fx-basic-integration.md)** | FX 依赖注入基础集成指南 | ⭐⭐ | 入门级 FX 项目 |
| **[fx-advanced-guide.md](./fx-advanced-guide.md)** | FX 高级实战技能 | ⭐⭐⭐⭐ | 中大型企业项目 |
| **[fx-simplified-architecture.md](./fx-simplified-architecture.md)** | FX 简化架构实战（Handler 自管理路由） | ⭐⭐⭐ | MVP 快速开发 |

#### 参考资料

- **[references/fx-architecture.md](./references/fx-architecture.md)** - 完整的 FX 架构实现示例
- **[references/patterns.md](./references/patterns.md)** - Go 设计模式
- **[references/stdlib.md](./references/stdlib.md)** - 标准库使用指南
- **[references/tools.md](./references/tools.md)** - 开发工具配置

### 🚀 快速开始

#### 1. 克隆仓库

```bash
git clone https://github.com/difyz9/golang-best-practices.git
cd golang-best-practices
```

#### 2. 选择适合你的技能文档

**新手入门路线**:
```
SKILL.md → fx-basic-integration.md → fx-simplified-architecture.md
```

**进阶路线**:
```
SKILL.md → fx-basic-integration.md → fx-advanced-guide.md → references/
```

**快速开发路线**:
```
fx-simplified-architecture.md (Handler 自管理模式)
```

### 📁 项目结构

```
golang-best-practices/
├── README.md                          # 本文件
├── SKILL.md                           # Golang 最佳实践核心文档
├── fx-basic-integration.md            # FX 基础集成指南
├── fx-advanced-guide.md               # FX 高级实战指南
├── fx-simplified-architecture.md      # FX 简化架构指南
├── references/                 # 详细参考资料
│   ├── fx-architecture.md
│   ├── patterns.md
│   ├── stdlib.md
│   └── tools.md
├── examples/                   # 代码示例（可选）
│   ├── basic-fx/
│   ├── advanced-fx/
│   └── simplified-fx/
└── .gitignore
```

### 💡 使用方式

#### 方式 1: 作为学习资料

直接阅读 Markdown 文档，按照示例代码进行实践。

#### 方式 2: 作为 Claude/Copilot Skill

将文档内容作为 AI 助手的技能包：

**Claude Desktop**:
```json
{
  "mcpServers": {
    "golang-skills": {
      "command": "cat",
      "args": ["/path/to/golang-best-practices/SKILL.md"]
    }
  }
}
```

**GitHub Copilot**:
将仓库添加到 `.claude/skills/` 或 `.copilot/skills/` 目录。

#### 方式 3: Git Submodule

在你的项目中作为子模块引用：

```bash
git submodule add https://github.com/difyz9/golang-best-practices.git docs/skills
```

### 🎓 学习路径建议

#### 阶段 1: 基础 (1-2 周)
- [ ] 阅读 `SKILL.md` 的项目结构、错误处理、并发章节
- [ ] 学习 `fx-basic-integration.md` 的 FX 基础概念
- [ ] 实践一个简单的 Gin + FX 项目

#### 阶段 2: 进阶 (2-4 周)
- [ ] 深入学习 `fx-advanced-guide.md` 的分层架构
- [ ] 掌握 Handler/Service/Repository 三层设计
- [ ] 实践路由模块化管理

#### 阶段 3: 实战 (持续)
- [ ] 参考 `fx-simplified-architecture.md` 实现快速迭代项目
- [ ] 学习 `references/` 中的设计模式
- [ ] 优化现有项目架构

### 🔧 技术栈

- **核心框架**: [Uber FX](https://uber-go.github.io/fx/)
- **Web 框架**: [Gin](https://gin-gonic.com/)
- **ORM**: [GORM](https://gorm.io/)
- **配置管理**: [Viper](https://github.com/spf13/viper)
- **日志**: [Zap](https://github.com/uber-go/zap)

### 🤝 贡献

欢迎提交 PR 改进文档！请遵循以下步骤：

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-skill`)
3. 提交更改 (`git commit -m 'Add some amazing skill'`)
4. 推送到分支 (`git push origin feature/amazing-skill`)
5. 创建 Pull Request

### 📜 许可证

本项目采用 Apache 2.0 许可证 - 详见 [LICENSE](LICENSE) 文件

### 🙏 致谢

- 源码参考: [yangjian102621/geekai](https://github.com/yangjian102621/geekai)
- Uber FX 团队提供的优秀依赖注入框架
- Go 社区的所有贡献者

### 📮 联系方式

- 提交 Issue: [GitHub Issues](https://github.com/difyz9/golang-best-practices/issues)
- 讨论: [GitHub Discussions](https://github.com/difyz9/golang-best-practices/discussions)

---

## English

### 📖 Introduction

This skill pack provides a comprehensive guide to Golang development best practices and Uber FX dependency injection framework, suitable for developers at all levels.

### 🎯 Contents

#### Core Skill Documents

| Document | Description | Difficulty | Use Case |
|----------|-------------|------------|----------|
| **[SKILL.md](./SKILL.md)** | Complete Golang Best Practices | ⭐⭐⭐ | All Go Projects |
| **[fx-basic-integration.md](./fx-basic-integration.md)** | FX Basic Integration Guide | ⭐⭐ | Entry-level FX Projects |
| **[fx-advanced-guide.md](./fx-advanced-guide.md)** | FX Advanced Skills | ⭐⭐⭐⭐ | Enterprise Projects |
| **[fx-simplified-architecture.md](./fx-simplified-architecture.md)** | FX Simplified Architecture (Self-Managed Routes) | ⭐⭐⭐ | MVP Fast Development |

#### References

- **[references/fx-architecture.md](./references/fx-architecture.md)** - Complete FX Architecture Examples
- **[references/patterns.md](./references/patterns.md)** - Go Design Patterns
- **[references/stdlib.md](./references/stdlib.md)** - Standard Library Guide
- **[references/tools.md](./references/tools.md)** - Development Tools Setup

### 🚀 Quick Start

#### 1. Clone Repository

```bash
git clone https://github.com/difyz9/golang-best-practices.git
cd golang-best-practices
```

#### 2. Choose Your Learning Path

**Beginner Path**:
```
SKILL.md → fx-basic-integration.md → fx-simplified-architecture.md
```

**Advanced Path**:
```
SKILL.md → fx-basic-integration.md → fx-advanced-guide.md → references/
```

**Fast Development Path**:
```
fx-simplified-architecture.md (Handler Self-Managed Pattern)
```

### 💡 Usage

#### Method 1: Learning Material

Read Markdown documents directly and practice with example code.

#### Method 2: As Claude/Copilot Skill

Use documents as AI assistant skill pack.

#### Method 3: Git Submodule

Reference as submodule in your project:

```bash
git submodule add https://github.com/difyz9/golang-best-practices.git docs/skills
```

### 📜 License

Apache 2.0 License - see [LICENSE](LICENSE) file

### 🙏 Acknowledgments

- Source Reference: [yangjian102621/geekai](https://github.com/yangjian102621/geekai)
- Uber FX Team
- Go Community

---

<div align="center">
Made with ❤️ by Golang Community
</div>
