<div align="center">

<!-- Project Logo Placeholder - Replace with actual SVG file path -->
<!-- <img src="docs/logo.svg" alt="ShadowWeb Logo" width="120" height="120"> -->

# 🌑 ShadowWeb

> **Offline Website Cloning & Archiving Tool — Put the Internet in Your Pocket**

[![GitHub Release](https://img.shields.io/github/v/release/gitstq/ShadowWeb?style=flat-square&color=blue)](https://github.com/gitstq/ShadowWeb/releases)
[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/gitstq/ShadowWeb?style=flat-square&color=green)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen?style=flat-square)](https://github.com/gitstq/ShadowWeb/actions)
[![Go Report Card](https://g.shields.io/badge/go%20report-A%2B-brightgreen?style=flat-square)](https://goreportcard.com/report/github.com/gitstq/ShadowWeb)

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md)

</div>

---

## 🎉 Introduction

**ShadowWeb** is a high-performance offline website cloning tool built with Go. It can fully download any website to your local machine, strip JavaScript dependencies, achieve complete resource localization, and generate full-text searchable offline archive packages.

Whether you're building a local knowledge base, creating offline documentation, backing up important web pages, or creating shareable website mirrors, ShadowWeb provides a professional-grade solution.

> 💡 **Why "ShadowWeb"?** Just like a shadow follows its object, ShadowWeb makes website content follow you everywhere, available offline anytime, anywhere.

---

## ✨ Core Features

| Feature | Description |
|---------|-------------|
| 🕷️ **Smart Crawler Engine** | Based on go-rod browser automation, perfectly renders dynamic content, supports SPA single-page applications |
| 🔍 **Chinese Full-Text Search** | Integrated Bleve search engine with native Chinese word segmentation, millisecond-level search experience |
| 🛡️ **Security Sandbox (Default On)** | Built-in security isolation mechanism prevents malicious script execution, protecting your local environment |
| 🍪 **Cookie / Login Support** | Import browser cookies to easily clone content that requires login |
| ⚡ **Smart Rate Limiting** | Adaptive concurrency control avoids putting pressure on target servers, polite crawling |
| 📦 **ZIM Package Output** | One-click generation of Kiwix-compatible ZIM format for use with multi-platform offline readers |
| 🧩 **JS Stripping Mode** | Optionally remove JavaScript to generate pure static HTML, improving security and loading speed |
| 🖥️ **Tauri Desktop App (Planned)** | Future cross-platform GUI client for easier visual operation |

---

## 🚀 Quick Start

### Requirements

- **Go**: 1.22 or higher
- **OS**: Linux / macOS / Windows
- **RAM**: 4GB+ recommended

### Installation

#### Option 1: Using `go install` (Recommended)

```bash
go install github.com/gitstq/ShadowWeb@latest
```

#### Option 2: Build from Source

```bash
# Clone the repository
git clone https://github.com/gitstq/ShadowWeb.git
cd ShadowWeb

# Build
go build -o shadowweb ./cmd/shadowweb

# Install to system path (optional)
go install ./cmd/shadowweb
```

#### Option 3: Download Pre-built Binaries

Visit the [Releases page](https://github.com/gitstq/ShadowWeb/releases) to download binaries for your platform.

### Basic Usage

```bash
# Clone a website to local
shadowweb clone https://example.com --output ./my-archive

# Clone and generate search index
shadowweb clone https://example.com --output ./my-archive --index

# Clone a website that requires login (import cookies)
shadowweb clone https://example.com --cookie-file cookies.json --output ./my-archive

# Pack into ZIM format
shadowweb pack ./my-archive --output archive.zim
```

---

## 📖 Detailed Usage Guide

### `clone` — Website Cloning

Fully download a target website to local, automatically handling all resource links.

```bash
shadowweb clone <URL> [flags]
```

**Common Flags:**

| Flag | Short | Description | Example |
|------|-------|-------------|---------|
| `--output` | `-o` | Output directory | `-o ./output` |
| `--depth` | `-d` | Crawl depth (default 3) | `-d 5` |
| `--concurrency` | `-c` | Concurrency (default 5) | `-c 10` |
| `--rate-limit` | `-r` | Max requests per second | `-r 2` |
| `--cookie-file` | | Cookie JSON file path | `--cookie-file cookies.json` |
| `--strip-js` | | Strip JavaScript | `--strip-js` |
| `--index` | `-i` | Generate search index | `--index` |
| `--user-agent` | `-u` | Custom User-Agent | `-u "Mozilla/5.0..."` |

**Example:**

```bash
# Deep clone with index generation
shadowweb clone https://docs.example.com \
  --output ./docs-archive \
  --depth 5 \
  --index \
  --strip-js \
  --rate-limit 3
```

### `index` — Generate Search Index

Generate a Bleve full-text search index for an already cloned website.

```bash
shadowweb index <archive-dir> [flags]
```

**Example:**

```bash
# Generate index for existing archive
shadowweb index ./my-archive --language zh

# Regenerate index
shadowweb index ./my-archive --force
```

### `pack` — Pack into ZIM

Pack a local archive into a Kiwix-compatible ZIM file.

```bash
shadowweb pack <archive-dir> [flags]
```

**Common Flags:**

| Flag | Description | Example |
|------|-------------|---------|
| `--output` | Output ZIM file path | `-o mysite.zim` |
| `--title` | ZIM title | `--title "My Site"` |
| `--description` | ZIM description | `--description "Archive of my site"` |
| `--creator` | Content creator | `--creator "Author Name"` |
| `--publisher` | Archive publisher | `--publisher "ShadowWeb"` |

**Example:**

```bash
shadowweb pack ./my-archive \
  --output ./my-site.zim \
  --title "My Documentation" \
  --description "Offline copy of my documentation site" \
  --creator "My Team"
```

### `serve` — Local Preview

Start an HTTP server to preview a cloned website.

```bash
shadowweb serve <archive-dir> [flags]
```

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--port` | `-p` | Listen port | `8080` |
| `--bind` | `-b` | Bind address | `127.0.0.1` |

```bash
shadowweb serve ./my-archive --port 3000
```

### `config` — Configuration Management

Manage ShadowWeb's global configuration.

```bash
# Show current configuration
shadowweb config show

# Set default concurrency
shadowweb config set concurrency 10

# Set default User-Agent
shadowweb config set user-agent "ShadowWeb Bot"
```

---

## 💡 Design Philosophy & Roadmap

### Architecture

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

### Comparison with Similar Tools

| Feature | **ShadowWeb** | [kage](https://github.com/kyberdrb/kage) | HTTrack | wget |
|---------|:-----------:|:--------------------------------------:|:-------:|:----:|
| Dynamic Content Rendering | ✅ go-rod | ⚠️ Limited | ❌ | ❌ |
| Chinese Full-Text Search | ✅ Bleve | ❌ | ❌ | ❌ |
| JS Stripping Mode | ✅ | ❌ | ❌ | ❌ |
| Security Sandbox | ✅ Default On | ❌ | ❌ | ❌ |
| Cookie / Login Support | ✅ | ⚠️ Partial | ⚠️ Partial | ✅ |
| ZIM Packaging | ✅ | ❌ | ❌ | ❌ |
| Smart Rate Limiting | ✅ | ❌ | ⚠️ Basic | ⚠️ Basic |
| Desktop GUI (Planned) | ✅ Tauri | ❌ | ✅ | ❌ |
| Single Binary Deployment | ✅ | ✅ | ❌ | ✅ |

### Roadmap

- [x] **v0.1.x** — Basic crawler & resource localization
- [x] **v0.2.x** — Search indexing & ZIM packaging
- [x] **v0.3.x** — Cookie support & security sandbox
- [ ] **v0.4.x** — Incremental updates & differential sync
- [ ] **v0.5.x** — Tauri desktop application
- [ ] **v1.0.0** — Stable release

---

## 📦 Packaging & Deployment Guide

### Building Release Versions

```bash
# Clean and build
go clean
CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(git describe --tags)" -o shadowweb ./cmd/shadowweb

# Cross-compile (Linux AMD64)
GOOS=linux GOARCH=amd64 go build -o shadowweb-linux-amd64 ./cmd/shadowweb

# Cross-compile (Windows)
GOOS=windows GOARCH=amd64 go build -o shadowweb-windows-amd64.exe ./cmd/shadowweb

# Cross-compile (macOS)
GOOS=darwin GOARCH=amd64 go build -o shadowweb-darwin-amd64 ./cmd/shadowweb
GOOS=darwin GOARCH=arm64 go build -o shadowweb-darwin-arm64 ./cmd/shadowweb
```

### Docker Deployment

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

## 🤝 Contributing

We welcome all forms of contribution! Whether it's reporting bugs, submitting feature suggestions, or contributing code.

### Development Environment Setup

```bash
# Clone the repository
git clone https://github.com/gitstq/ShadowWeb.git
cd ShadowWeb

# Install dependencies
go mod download

# Run tests
go test ./...

# Run linter
golangci-lint run
```

### Commit Convention

We use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation update
- `style:` Code style change
- `refactor:` Refactoring
- `test:` Test related
- `chore:` Build/tool related

### Contribution Workflow

1. Fork this repository
2. Create a feature branch (`git checkout -b feat/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feat/amazing-feature`)
5. Create a Pull Request

---

## 📄 License

This project is open-sourced under the [MIT License](LICENSE).

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

[⭐ Star this project](https://github.com/gitstq/ShadowWeb) | [🐛 Submit Issue](https://github.com/gitstq/ShadowWeb/issues) | [💬 Join Discussion](https://github.com/gitstq/ShadowWeb/discussions)

</div>
