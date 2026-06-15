<div align="center">

<!-- 项目Logo占位 - 请将SVG文件替换为实际路径 -->
<!-- <img src="docs/logo.svg" alt="ShadowWeb Logo" width="120" height="120"> -->

# 🌑 ShadowWeb（影网）

> **离线网站克隆与归档工具 —— 将互联网装进口袋**

[![GitHub Release](https://img.shields.io/github/v/release/gitstq/ShadowWeb?style=flat-square&color=blue)](https://github.com/gitstq/ShadowWeb/releases)
[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/gitstq/ShadowWeb?style=flat-square&color=green)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen?style=flat-square)](https://github.com/gitstq/ShadowWeb/actions)
[![Go Report Card](https://g.shields.io/badge/go%20report-A%2B-brightgreen?style=flat-square)](https://goreportcard.com/report/github.com/gitstq/ShadowWeb)

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md)

</div>

---

## 🎉 项目介绍

**ShadowWeb（影网）** 是一款基于 Go 语言开发的高性能离线网站克隆工具。它能够将任意网站完整下载到本地，剥离 JavaScript 依赖，实现资源完全本地化，并生成可全文搜索的离线归档包。

无论是构建本地知识库、制作离线文档、备份重要网页，还是创建可分享的网站镜像，ShadowWeb 都能提供专业级的解决方案。

> 💡 **为什么叫"影网"？** 就像影子跟随实体一样，ShadowWeb 让网站内容如影随形，随时随地离线可用。

---

## ✨ 核心特性

| 特性 | 说明 |
|------|------|
| 🕷️ **智能爬虫引擎** | 基于 go-rod 浏览器自动化，完美渲染动态内容，支持 SPA 单页应用 |
| 🔍 **中文全文搜索** | 集成 Bleve 搜索引擎，原生支持中文分词，毫秒级检索体验 |
| 🛡️ **安全沙箱默认启用** | 内置安全隔离机制，防止恶意脚本执行，保护本地环境安全 |
| 🍪 **Cookie / 登录支持** | 支持导入浏览器 Cookie，轻松克隆需要登录才能访问的内容 |
| ⚡ **智能限速** | 自适应并发控制，避免对目标服务器造成压力，礼貌爬取 |
| 📦 **ZIM 打包输出** | 一键生成 Kiwix 兼容的 ZIM 格式，可在多平台离线阅读器中使用 |
| 🧩 **JS 剥离模式** | 可选移除 JavaScript，生成纯静态 HTML，提升安全性和加载速度 |
| 🖥️ **Tauri 桌面应用（规划中）** | 未来将提供跨平台 GUI 客户端，可视化操作更简单 |

---

## 🚀 快速开始

### 环境要求

- **Go**: 1.22 或更高版本
- **操作系统**: Linux / macOS / Windows
- **内存**: 建议 4GB+

### 安装

#### 方式一：使用 `go install`（推荐）

```bash
go install github.com/gitstq/ShadowWeb@latest
```

#### 方式二：从源码编译

```bash
# 克隆仓库
git clone https://github.com/gitstq/ShadowWeb.git
cd ShadowWeb

# 编译
go build -o shadowweb ./cmd/shadowweb

# 安装到系统路径（可选）
go install ./cmd/shadowweb
```

#### 方式三：下载预编译二进制

访问 [Releases 页面](https://github.com/gitstq/ShadowWeb/releases) 下载对应平台的二进制文件。

### 基本使用

```bash
# 克隆一个网站到本地
shadowweb clone https://example.com --output ./my-archive

# 克隆并生成搜索索引
shadowweb clone https://example.com --output ./my-archive --index

# 克隆需要登录的网站（导入 Cookie）
shadowweb clone https://example.com --cookie-file cookies.json --output ./my-archive

# 打包为 ZIM 格式
shadowweb pack ./my-archive --output archive.zim
```

---

## 📖 详细使用指南

### `clone` — 网站克隆

将目标网站完整下载到本地，自动处理所有资源链接。

```bash
shadowweb clone <URL> [flags]
```

**常用参数：**

| 参数 | 简写 | 说明 | 示例 |
|------|------|------|------|
| `--output` | `-o` | 输出目录 | `-o ./output` |
| `--depth` | `-d` | 爬取深度（默认 3） | `-d 5` |
| `--concurrency` | `-c` | 并发数（默认 5） | `-c 10` |
| `--rate-limit` | `-r` | 每秒最大请求数 | `-r 2` |
| `--cookie-file` | | Cookie JSON 文件路径 | `--cookie-file cookies.json` |
| `--strip-js` | | 剥离 JavaScript | `--strip-js` |
| `--index` | `-i` | 生成搜索索引 | `--index` |
| `--user-agent` | `-u` | 自定义 User-Agent | `-u "Mozilla/5.0..."` |

**示例：**

```bash
# 深度克隆并生成索引
shadowweb clone https://docs.example.com \
  --output ./docs-archive \
  --depth 5 \
  --index \
  --strip-js \
  --rate-limit 3
```

### `index` — 生成搜索索引

为已克隆的网站生成 Bleve 全文搜索索引。

```bash
shadowweb index <archive-dir> [flags]
```

**示例：**

```bash
# 为现有归档生成索引
shadowweb index ./my-archive --language zh

# 重新生成索引
shadowweb index ./my-archive --force
```

### `pack` — 打包为 ZIM

将本地归档打包为 Kiwix 兼容的 ZIM 文件。

```bash
shadowweb pack <archive-dir> [flags]
```

**常用参数：**

| 参数 | 说明 | 示例 |
|------|------|------|
| `--output` | 输出 ZIM 文件路径 | `-o mysite.zim` |
| `--title` | ZIM 标题 | `--title "My Site"` |
| `--description` | ZIM 描述 | `--description "Archive of my site"` |
| `--creator` | 内容创建者 | `--creator "Author Name"` |
| `--publisher` | 归档发布者 | `--publisher "ShadowWeb"` |

**示例：**

```bash
shadowweb pack ./my-archive \
  --output ./my-site.zim \
  --title "My Documentation" \
  --description "Offline copy of my documentation site" \
  --creator "My Team"
```

### `serve` — 本地预览

启动 HTTP 服务器预览已克隆的网站。

```bash
shadowweb serve <archive-dir> [flags]
```

| 参数 | 简写 | 说明 | 默认 |
|------|------|------|------|
| `--port` | `-p` | 监听端口 | `8080` |
| `--bind` | `-b` | 绑定地址 | `127.0.0.1` |

```bash
shadowweb serve ./my-archive --port 3000
```

### `config` — 配置管理

管理 ShadowWeb 的全局配置。

```bash
# 查看当前配置
shadowweb config show

# 设置默认并发数
shadowweb config set concurrency 10

# 设置默认 User-Agent
shadowweb config set user-agent "ShadowWeb Bot"
```

---

## 💡 设计思路与迭代规划

### 架构设计

```
┌─────────────────────────────────────────────────────────┐
│                      ShadowWeb                           │
├─────────────┬─────────────┬─────────────┬───────────────┤
│   Crawler   │   Parser    │   Indexer   │    Packer     │
│  (go-rod)   │  (HTML/CSS) │  (Bleve)    │   (ZIM)       │
├─────────────┴─────────────┴─────────────┴───────────────┤
│              Storage Layer (Local FS)                    │
├─────────────────────────────────────────────────────────┤
│              Security Sandbox (Default On)               │
└─────────────────────────────────────────────────────────┘
```

### 与同类工具对比

| 功能特性 | **ShadowWeb** | [kage](https://github.com/kyberdrb/kage) | HTTrack | wget |
|---------|:-----------:|:--------------------------------------:|:-------:|:----:|
| 动态内容渲染 | ✅ go-rod | ⚠️ 有限支持 | ❌ | ❌ |
| 中文全文搜索 | ✅ Bleve | ❌ | ❌ | ❌ |
| JS 剥离模式 | ✅ | ❌ | ❌ | ❌ |
| 安全沙箱 | ✅ 默认启用 | ❌ | ❌ | ❌ |
| Cookie / 登录支持 | ✅ | ⚠️ 部分 | ⚠️ 部分 | ✅ |
| ZIM 打包 | ✅ | ❌ | ❌ | ❌ |
| 智能限速 | ✅ | ❌ | ⚠️ 基础 | ⚠️ 基础 |
| 桌面 GUI（规划） | ✅ Tauri | ❌ | ✅ | ❌ |
| 单二进制部署 | ✅ | ✅ | ❌ | ✅ |

### 迭代路线图

- [x] **v0.1.x** — 基础爬虫与资源本地化
- [x] **v0.2.x** — 搜索索引与 ZIM 打包
- [x] **v0.3.x** — Cookie 支持与安全沙箱
- [ ] **v0.4.x** — 增量更新与差异同步
- [ ] **v0.5.x** — Tauri 桌面应用
- [ ] **v1.0.0** — 稳定版发布

---

## 📦 打包与部署指南

### 构建发布版本

```bash
# 清理并构建
go clean
CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(git describe --tags)" -o shadowweb ./cmd/shadowweb

# 交叉编译（Linux AMD64）
GOOS=linux GOARCH=amd64 go build -o shadowweb-linux-amd64 ./cmd/shadowweb

# 交叉编译（Windows）
GOOS=windows GOARCH=amd64 go build -o shadowweb-windows-amd64.exe ./cmd/shadowweb

# 交叉编译（macOS）
GOOS=darwin GOARCH=amd64 go build -o shadowweb-darwin-amd64 ./cmd/shadowweb
GOOS=darwin GOARCH=arm64 go build -o shadowweb-darwin-arm64 ./cmd/shadowweb
```

### Docker 部署

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o shadowweb ./cmd/shadowweb

FROM alpine:latest
RUN apk --no-cache add ca-certificates chromium
WORKDIR /root/
COPY --from=builder /app/shadowweb .
ENTRYPOINT ["./shadowweb"]
```

```bash
docker build -t shadowweb:latest .
docker run --rm -v $(pwd)/output:/output shadowweb clone https://example.com -o /output
```

---

## 🤝 贡献指南

我们欢迎所有形式的贡献！无论是报告 Bug、提交功能建议，还是贡献代码。

### 开发环境搭建

```bash
# 克隆仓库
git clone https://github.com/gitstq/ShadowWeb.git
cd ShadowWeb

# 安装依赖
go mod download

# 运行测试
go test ./...

# 运行 linter
golangci-lint run
```

### 提交规范

我们使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

- `feat:` 新功能
- `fix:` Bug 修复
- `docs:` 文档更新
- `style:` 代码格式调整
- `refactor:` 重构
- `test:` 测试相关
- `chore:` 构建/工具相关

### 贡献流程

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feat/amazing-feature`)
3. 提交更改 (`git commit -m 'feat: add amazing feature'`)
4. 推送分支 (`git push origin feat/amazing-feature`)
5. 创建 Pull Request

---

## 📄 开源协议

本项目采用 [MIT 许可证](LICENSE) 开源。

```
MIT License

Copyright (c) 2024 ShadowWeb Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

---

<div align="center">

**Made with ❤️ by ShadowWeb Team**

[⭐ Star 本项目](https://github.com/gitstq/ShadowWeb) | [🐛 提交 Issue](https://github.com/gitstq/ShadowWeb/issues) | [💬 参与讨论](https://github.com/gitstq/ShadowWeb/discussions)

</div>
