<div align="center">

<!-- 專案Logo占位 - 請將SVG檔案替換為實際路徑 -->
<!-- <img src="docs/logo.svg" alt="ShadowWeb Logo" width="120" height="120"> -->

# 🌑 ShadowWeb（影網）

> **離線網站複製與歸檔工具 —— 將網際網路裝進口袋**

[![GitHub Release](https://img.shields.io/github/v/release/gitstq/ShadowWeb?style=flat-square&color=blue)](https://github.com/gitstq/ShadowWeb/releases)
[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/gitstq/ShadowWeb?style=flat-square&color=green)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen?style=flat-square)](https://github.com/gitstq/ShadowWeb/actions)
[![Go Report Card](https://g.shields.io/badge/go%20report-A%2B-brightgreen?style=flat-square)](https://goreportcard.com/report/github.com/gitstq/ShadowWeb)

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md)

</div>

---

## 🎉 專案介紹

**ShadowWeb（影網）** 是一款基於 Go 語言開發的高效能離線網站複製工具。它能夠將任意網站完整下載到本地，剝離 JavaScript 依賴，實現資源完全在地化，並生成可全文搜尋的離線歸檔包。

無論是建構本地知識庫、製作離線文件、備份重要網頁，還是建立可分享的網站鏡像，ShadowWeb 都能提供專業級的解決方案。

> 💡 **為什麼叫「影網」？** 就像影子跟隨實體一樣，ShadowWeb 讓網站內容如影隨形，隨時隨地離線可用。

---

## ✨ 核心特性

| 特性 | 說明 |
|------|------|
| 🕷️ **智慧爬蟲引擎** | 基於 go-rod 瀏覽器自動化，完美渲染動態內容，支援 SPA 單頁應用 |
| 🔍 **中文全文搜尋** | 整合 Bleve 搜尋引擎，原生支援中文分詞，毫秒級檢索體驗 |
| 🛡️ **安全沙箱預設啟用** | 內建安全隔離機制，防止惡意指令碼執行，保護本地環境安全 |
| 🍪 **Cookie / 登入支援** | 支援匯入瀏覽器 Cookie，輕鬆複製需要登入才能存取的內容 |
| ⚡ **智慧限速** | 自適應並發控制，避免對目標伺服器造成壓力，禮貌爬取 |
| 📦 **ZIM 打包輸出** | 一鍵生成 Kiwix 相容的 ZIM 格式，可在多平台離線閱讀器中使用 |
| 🧩 **JS 剝離模式** | 可選移除 JavaScript，生成純靜態 HTML，提升安全性和載入速度 |
| 🖥️ **Tauri 桌面應用（規劃中）** | 未來將提供跨平台 GUI 客戶端，視覺化操作更簡單 |

---

## 🚀 快速開始

### 環境要求

- **Go**: 1.22 或更高版本
- **作業系統**: Linux / macOS / Windows
- **記憶體**: 建議 4GB+

### 安裝

#### 方式一：使用 `go install`（推薦）

```bash
go install github.com/gitstq/ShadowWeb@latest
```

#### 方式二：從原始碼編譯

```bash
# 複製倉庫
git clone https://github.com/gitstq/ShadowWeb.git
cd ShadowWeb

# 編譯
go build -o shadowweb ./cmd/shadowweb

# 安裝到系統路徑（可選）
go install ./cmd/shadowweb
```

#### 方式三：下載預編譯二進位檔

訪問 [Releases 頁面](https://github.com/gitstq/ShadowWeb/releases) 下載對應平台的二進位檔案。

### 基本使用

```bash
# 複製一個網站到本地
shadowweb clone https://example.com --output ./my-archive

# 複製並生成搜尋索引
shadowweb clone https://example.com --output ./my-archive --index

# 複製需要登入的網站（匯入 Cookie）
shadowweb clone https://example.com --cookie-file cookies.json --output ./my-archive

# 打包為 ZIM 格式
shadowweb pack ./my-archive --output archive.zim
```

---

## 📖 詳細使用指南

### `clone` — 網站複製

將目標網站完整下載到本地，自動處理所有資源連結。

```bash
shadowweb clone <URL> [flags]
```

**常用參數：**

| 參數 | 簡寫 | 說明 | 範例 |
|------|------|------|------|
| `--output` | `-o` | 輸出目錄 | `-o ./output` |
| `--depth` | `-d` | 爬取深度（預設 3） | `-d 5` |
| `--concurrency` | `-c` | 並發數（預設 5） | `-c 10` |
| `--rate-limit` | `-r` | 每秒最大請求數 | `-r 2` |
| `--cookie-file` | | Cookie JSON 檔案路徑 | `--cookie-file cookies.json` |
| `--strip-js` | | 剝離 JavaScript | `--strip-js` |
| `--index` | `-i` | 生成搜尋索引 | `--index` |
| `--user-agent` | `-u` | 自訂 User-Agent | `-u "Mozilla/5.0..."` |

**範例：**

```bash
# 深度複製並生成索引
shadowweb clone https://docs.example.com \
  --output ./docs-archive \
  --depth 5 \
  --index \
  --strip-js \
  --rate-limit 3
```

### `index` — 生成搜尋索引

為已複製的網站生成 Bleve 全文搜尋索引。

```bash
shadowweb index <archive-dir> [flags]
```

**範例：**

```bash
# 為現有歸檔生成索引
shadowweb index ./my-archive --language zh

# 重新生成索引
shadowweb index ./my-archive --force
```

### `pack` — 打包為 ZIM

將本地歸檔打包為 Kiwix 相容的 ZIM 檔案。

```bash
shadowweb pack <archive-dir> [flags]
```

**常用參數：**

| 參數 | 說明 | 範例 |
|------|------|------|
| `--output` | 輸出 ZIM 檔案路徑 | `-o mysite.zim` |
| `--title` | ZIM 標題 | `--title "My Site"` |
| `--description` | ZIM 描述 | `--description "Archive of my site"` |
| `--creator` | 內容建立者 | `--creator "Author Name"` |
| `--publisher` | 歸檔發布者 | `--publisher "ShadowWeb"` |

**範例：**

```bash
shadowweb pack ./my-archive \
  --output ./my-site.zim \
  --title "My Documentation" \
  --description "Offline copy of my documentation site" \
  --creator "My Team"
```

### `serve` — 本地預覽

啟動 HTTP 伺服器預覽已複製的網站。

```bash
shadowweb serve <archive-dir> [flags]
```

| 參數 | 簡寫 | 說明 | 預設 |
|------|------|------|------|
| `--port` | `-p` | 監聽埠 | `8080` |
| `--bind` | `-b` | 綁定位址 | `127.0.0.1` |

```bash
shadowweb serve ./my-archive --port 3000
```

### `config` — 配置管理

管理 ShadowWeb 的全域配置。

```bash
# 檢視目前配置
shadowweb config show

# 設定預設並發數
shadowweb config set concurrency 10

# 設定預設 User-Agent
shadowweb config set user-agent "ShadowWeb Bot"
```

---

## 💡 設計思路與迭代規劃

### 架構設計

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

### 與同類工具對比

| 功能特性 | **ShadowWeb** | [kage](https://github.com/kyberdrb/kage) | HTTrack | wget |
|---------|:-----------:|:--------------------------------------:|:-------:|:----:|
| 動態內容渲染 | ✅ go-rod | ⚠️ 有限支援 | ❌ | ❌ |
| 中文全文搜尋 | ✅ Bleve | ❌ | ❌ | ❌ |
| JS 剝離模式 | ✅ | ❌ | ❌ | ❌ |
| 安全沙箱 | ✅ 預設啟用 | ❌ | ❌ | ❌ |
| Cookie / 登入支援 | ✅ | ⚠️ 部分 | ⚠️ 部分 | ✅ |
| ZIM 打包 | ✅ | ❌ | ❌ | ❌ |
| 智慧限速 | ✅ | ❌ | ⚠️ 基礎 | ⚠️ 基礎 |
| 桌面 GUI（規劃） | ✅ Tauri | ❌ | ✅ | ❌ |
| 單二進位部署 | ✅ | ✅ | ❌ | ✅ |

### 迭代路線圖

- [x] **v0.1.x** — 基礎爬蟲與資源在地化
- [x] **v0.2.x** — 搜尋索引與 ZIM 打包
- [x] **v0.3.x** — Cookie 支援與安全沙箱
- [ ] **v0.4.x** — 增量更新與差異同步
- [ ] **v0.5.x** — Tauri 桌面應用
- [ ] **v1.0.0** — 穩定版發布

---

## 📦 打包與部署指南

### 構建發布版本

```bash
# 清理並構建
go clean
CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(git describe --tags)" -o shadowweb ./cmd/shadowweb

# 交叉編譯（Linux AMD64）
GOOS=linux GOARCH=amd64 go build -o shadowweb-linux-amd64 ./cmd/shadowweb

# 交叉編譯（Windows）
GOOS=windows GOARCH=amd64 go build -o shadowweb-windows-amd64.exe ./cmd/shadowweb

# 交叉編譯（macOS）
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

## 🤝 貢獻指南

我們歡迎所有形式的貢獻！無論是回報 Bug、提交功能建議，還是貢獻程式碼。

### 開發環境搭建

```bash
# 複製倉庫
git clone https://github.com/gitstq/ShadowWeb.git
cd ShadowWeb

# 安裝依賴
go mod download

# 執行測試
go test ./...

# 執行 linter
golangci-lint run
```

### 提交規範

我們使用 [Conventional Commits](https://www.conventionalcommits.org/) 規範：

- `feat:` 新功能
- `fix:` Bug 修復
- `docs:` 文件更新
- `style:` 程式碼格式調整
- `refactor:` 重構
- `test:` 測試相關
- `chore:` 構建/工具相關

### 貢獻流程

1. Fork 本倉庫
2. 建立功能分支 (`git checkout -b feat/amazing-feature`)
3. 提交更改 (`git commit -m 'feat: add amazing feature'`)
4. 推送分支 (`git push origin feat/amazing-feature`)
5. 建立 Pull Request

---

## 📄 開源協議

本專案採用 [MIT 授權條款](LICENSE) 開源。

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

[⭐ Star 本專案](https://github.com/gitstq/ShadowWeb) | [🐛 提交 Issue](https://github.com/gitstq/ShadowWeb/issues) | [💬 參與討論](https://github.com/gitstq/ShadowWeb/discussions)

</div>
