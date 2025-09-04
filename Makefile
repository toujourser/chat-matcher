# Go项目编译Makefile
# 支持Linux和Mac的x86和ARM平台

# 项目信息
PROJECT_NAME := chat-matcher
MODULE_NAME := github.com/toujourser/chat-matcher
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 构建目录
BUILD_DIR := build
DIST_DIR := dist

# Go编译参数
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.CommitHash=$(COMMIT_HASH) -w -s"
GO_BUILD := go build $(LDFLAGS)

# 目标平台
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64

# 默认目标
.PHONY: all
all: clean build

# 清理构建文件
.PHONY: clean
clean:
	@echo "清理构建文件..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@echo "清理完成"

# 创建构建目录
$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

$(DIST_DIR):
	@mkdir -p $(DIST_DIR)

# 构建所有平台
.PHONY: build
build: $(BUILD_DIR) build-server build-cli

# 构建服务器端所有平台
.PHONY: build-server
build-server: $(BUILD_DIR)
	@echo "构建服务器端所有平台..."
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output_name=$(PROJECT_NAME)-server-$$os-$$arch; \
		if [ "$$os" = "windows" ]; then \
			output_name=$$output_name.exe; \
		fi; \
		echo "构建 $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO_BUILD) -o $(BUILD_DIR)/$$output_name .; \
		if [ $$? -ne 0 ]; then \
			echo "构建 $$os/$$arch 失败"; \
			exit 1; \
		fi; \
	done
	@echo "服务器端构建完成"

# 构建CLI客户端所有平台
.PHONY: build-cli
build-cli: $(BUILD_DIR)
	@echo "构建CLI客户端所有平台..."
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output_name=$(PROJECT_NAME)-cli-$$os-$$arch; \
		if [ "$$os" = "windows" ]; then \
			output_name=$$output_name.exe; \
		fi; \
		echo "构建CLI $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO_BUILD) -o $(BUILD_DIR)/$$output_name ./cli; \
		if [ $$? -ne 0 ]; then \
			echo "构建CLI $$os/$$arch 失败"; \
			exit 1; \
		fi; \
	done
	@echo "CLI客户端构建完成"

# 单独构建特定平台
.PHONY: build-linux-amd64
build-linux-amd64: $(BUILD_DIR)
	@echo "构建 Linux AMD64..."
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO_BUILD) -o $(BUILD_DIR)/$(PROJECT_NAME)-server-linux-amd64 .
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO_BUILD) -o $(BUILD_DIR)/$(PROJECT_NAME)-cli-linux-amd64 ./cli

.PHONY: build-linux-arm64
build-linux-arm64: $(BUILD_DIR)
	@echo "构建 Linux ARM64..."
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO_BUILD) -o $(BUILD_DIR)/$(PROJECT_NAME)-server-linux-arm64 .
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO_BUILD) -o $(BUILD_DIR)/$(PROJECT_NAME)-cli-linux-arm64 ./cli

.PHONY: build-darwin-amd64
build-darwin-amd64: $(BUILD_DIR)
	@echo "构建 macOS AMD64..."
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GO_BUILD) -o $(BUILD_DIR)/$(PROJECT_NAME)-server-darwin-amd64 .
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GO_BUILD) -o $(BUILD_DIR)/$(PROJECT_NAME)-cli-darwin-amd64 ./cli

.PHONY: build-darwin-arm64
build-darwin-arm64: $(BUILD_DIR)
	@echo "构建 macOS ARM64..."
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GO_BUILD) -o $(BUILD_DIR)/$(PROJECT_NAME)-server-darwin-arm64 .
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GO_BUILD) -o $(BUILD_DIR)/$(PROJECT_NAME)-cli-darwin-arm64 ./cli

# 本地构建（当前平台）
.PHONY: build-local
build-local: $(BUILD_DIR)
	@echo "构建本地版本..."
	@$(GO_BUILD) -o $(BUILD_DIR)/$(PROJECT_NAME)-server .
	@$(GO_BUILD) -o $(BUILD_DIR)/$(PROJECT_NAME)-cli ./cli
	@echo "本地构建完成"

# 打包发布版本
.PHONY: package
package: build $(DIST_DIR)
	@echo "打包发布版本..."
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		package_name=$(PROJECT_NAME)-$(VERSION)-$$os-$$arch; \
		mkdir -p $(DIST_DIR)/$$package_name; \
		cp $(BUILD_DIR)/$(PROJECT_NAME)-server-$$os-$$arch $(DIST_DIR)/$$package_name/; \
		cp $(BUILD_DIR)/$(PROJECT_NAME)-cli-$$os-$$arch $(DIST_DIR)/$$package_name/; \
		cp README.md $(DIST_DIR)/$$package_name/ 2>/dev/null || true; \
		cp LICENSE $(DIST_DIR)/$$package_name/ 2>/dev/null || true; \
		cd $(DIST_DIR) && tar -czf $$package_name.tar.gz $$package_name; \
		rm -rf $$package_name; \
		echo "已创建: $(DIST_DIR)/$$package_name.tar.gz"; \
	done
	@echo "打包完成"

# 运行服务器（本地）
.PHONY: run-server
run-server:
	@echo "启动服务器..."
	@go run .

# 运行CLI客户端（本地）
.PHONY: run-cli
run-cli:
	@echo "启动CLI客户端..."
	@go run ./cli

# 测试
.PHONY: test
test:
	@echo "运行测试..."
	@go test -v ./...

# 代码格式化
.PHONY: fmt
fmt:
	@echo "格式化代码..."
	@go fmt ./...

# 代码检查
.PHONY: vet
vet:
	@echo "代码检查..."
	@go vet ./...

# 依赖管理
.PHONY: mod-tidy
mod-tidy:
	@echo "整理依赖..."
	@go mod tidy

.PHONY: mod-download
mod-download:
	@echo "下载依赖..."
	@go mod download

# 显示构建信息
.PHONY: info
info:
	@echo "项目信息:"
	@echo "  项目名称: $(PROJECT_NAME)"
	@echo "  模块名称: $(MODULE_NAME)"
	@echo "  版本: $(VERSION)"
	@echo "  构建时间: $(BUILD_TIME)"
	@echo "  提交哈希: $(COMMIT_HASH)"
	@echo "  支持平台: $(PLATFORMS)"

# 显示帮助信息
.PHONY: help
help:
	@echo "可用的make目标:"
	@echo "  all              - 清理并构建所有平台"
	@echo "  build            - 构建所有平台"
	@echo "  build-server     - 构建服务器端所有平台"
	@echo "  build-cli        - 构建CLI客户端所有平台"
	@echo "  build-local      - 构建本地版本"
	@echo "  build-linux-amd64   - 构建Linux AMD64版本"
	@echo "  build-linux-arm64   - 构建Linux ARM64版本"
	@echo "  build-darwin-amd64  - 构建macOS AMD64版本"
	@echo "  build-darwin-arm64  - 构建macOS ARM64版本"
	@echo "  package          - 打包发布版本"
	@echo "  clean            - 清理构建文件"
	@echo "  run-server       - 运行服务器（本地）"
	@echo "  run-cli          - 运行CLI客户端（本地）"
	@echo "  test             - 运行测试"
	@echo "  fmt              - 格式化代码"
	@echo "  vet              - 代码检查"
	@echo "  mod-tidy         - 整理依赖"
	@echo "  mod-download     - 下载依赖"
	@echo "  info             - 显示构建信息"
	@echo "  help             - 显示此帮助信息"
