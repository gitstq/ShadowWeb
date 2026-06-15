.PHONY: all build clean test install lint fmt vet

BINARY_NAME=shadow
BUILD_DIR=bin
MODULE=github.com/gitstq/ShadowWeb

# 版本信息
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go 构建参数
LDFLAGS=-ldflags "-s -w \
	-X $(MODULE)/pkg/cli.Version=$(VERSION) \
	-X $(MODULE)/pkg/cli.BuildTime=$(BUILD_TIME) \
	-X $(MODULE)/pkg/cli.Commit=$(COMMIT)"

# 默认目标
all: build

# 构建二进制文件
build:
	@echo "正在构建 ShadowWeb..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/shadow
	@echo "构建完成: $(BUILD_DIR)/$(BINARY_NAME)"

# 交叉编译
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "构建 Linux 版本..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/shadow
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/shadow

build-darwin:
	@echo "构建 macOS 版本..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/shadow
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/shadow

build-windows:
	@echo "构建 Windows 版本..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/shadow

# 安装到 GOPATH/bin
install:
	@echo "安装 ShadowWeb..."
	CGO_ENABLED=0 go install $(LDFLAGS) ./cmd/shadow

# 运行测试
test:
	@echo "运行测试..."
	go test -v -race -coverprofile=coverage.out ./...

# 代码格式化
fmt:
	@echo "格式化代码..."
	go fmt ./...

# 静态分析
vet:
	@echo "运行 go vet..."
	go vet ./...

# 综合代码检查
lint: fmt vet
	@echo "代码检查完成"

# 清理构建产物
clean:
	@echo "清理构建产物..."
	@rm -rf $(BUILD_DIR)/
	@rm -f coverage.out

# 下载依赖
deps:
	@echo "下载依赖..."
	go mod download
	go mod tidy

# 运行示例
demo:
	@echo "运行示例（克隆 example.com）..."
	$(BUILD_DIR)/$(BINARY_NAME) clone https://example.com --output ./demo-output
