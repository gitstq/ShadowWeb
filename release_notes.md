## ShadowWeb v0.1.0 - 首个发布版本

ShadowWeb（影网）是一个高性能的离线网站克隆工具，基于 Headless Chrome 渲染页面，支持 JavaScript 剥离、资源本地化、全文搜索索引和 ZIM 格式打包。

### 核心功能

- **网站克隆 (`clone`)**：使用 Headless Chrome 渲染并克隆目标网站，支持双层 Worker 池高效并行下载
- **本地预览 (`serve`)**：启动本地 HTTP 服务器浏览克隆结果
- **离线打包 (`pack`)**：将克隆结果打包为 ZIM 或归档文件，便于分发和离线阅读
- **ZIM 阅读 (`open`)**：直接打开 ZIM 格式的离线包，启动本地服务器进行浏览
- **全文搜索 (`index`)**：使用 Bleve 搜索引擎为已克隆的 HTML 内容构建全文索引，支持中文内容

### 安装说明

#### 快速安装（Linux / macOS）

```bash
# 下载对应平台的二进制文件并解压
# Linux AMD64
curl -L -o shadow.tar.gz https://github.com/gitstq/ShadowWeb/releases/download/v0.1.0/shadow-linux-amd64.tar.gz
tar xzf shadow.tar.gz
sudo mv shadow /usr/local/bin/

# macOS Apple Silicon
curl -L -o shadow.tar.gz https://github.com/gitstq/ShadowWeb/releases/download/v0.1.0/shadow-darwin-arm64.tar.gz
tar xzf shadow.tar.gz
sudo mv shadow /usr/local/bin/
```

#### Windows

下载 `shadow-windows-amd64.zip`，解压后将 `shadow.exe` 添加到系统 PATH。

#### 从源码构建

```bash
git clone https://github.com/gitstq/ShadowWeb.git
cd ShadowWeb
make build
```

### 快速开始

```bash
# 克隆一个网站
shadow clone https://example.com --output ./example-output

# 启动本地服务器预览
shadow serve ./example-output --port 8080

# 打包为 ZIM 格式
shadow pack ./example-output --format zim

# 为克隆内容构建搜索索引
shadow index ./example-output

# 打开 ZIM 文件浏览
shadow open ./example-output.zim --port 8080
```

### 下载

| 平台 | 架构 | 下载 |
|------|------|------|
| Linux | AMD64 | shadow-linux-amd64.tar.gz |
| Linux | ARM64 | shadow-linux-arm64.tar.gz |
| macOS | AMD64 | shadow-darwin-amd64.tar.gz |
| macOS | ARM64 | shadow-darwin-arm64.tar.gz |
| Windows | AMD64 | shadow-windows-amd64.zip |
