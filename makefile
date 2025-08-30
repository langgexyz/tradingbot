PROJECT_NAME := tradingbot
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%Y-%m-%d-%H:%M:%S)
GO_VERSION := $(shell go version | awk '{print $$3}')

LDFLAGS := -X 'main.Version=$(VERSION)' \
           -X 'main.BuildTime=$(BUILD_TIME)' \
           -X 'main.GoVersion=$(GO_VERSION)'

.PHONY: help build build-linux build-windows build-macos build-all clean test fmt lint deps run run-backtest install

# 默认目标
help:
	@echo "布林道交易系统构建工具"
	@echo ""
	@echo "可用命令:"
	@echo "  build         构建当前平台可执行文件"
	@echo "  build-linux   构建 Linux 可执行文件"
	@echo "  build-windows 构建 Windows 可执行文件"
	@echo "  build-macos   构建 macOS 可执行文件"
	@echo "  build-all     构建所有平台可执行文件"
	@echo "  clean         清理构建文件"
	@echo "  test          运行测试"
	@echo "  fmt           格式化代码"
	@echo "  lint          代码静态检查"
	@echo "  deps          更新依赖"
	@echo "  run           运行程序"
	@echo "  run-backtest  运行回测示例"
	@echo "  install       安装到系统路径"

# 构建当前平台
build:
	@echo "构建 $(PROJECT_NAME) ..."
	@go mod tidy
	@go build -ldflags "$(LDFLAGS)" -o bin/$(PROJECT_NAME) src/main/main.go
	@echo "构建完成: bin/$(PROJECT_NAME)"

# 构建 Linux 版本
build-linux:
	@echo "构建 Linux 版本..."
	@go mod tidy
	@GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(PROJECT_NAME)-linux-amd64 src/main/main.go
	@echo "构建完成: bin/$(PROJECT_NAME)-linux-amd64"

# 构建 Windows 版本
build-windows:
	@echo "构建 Windows 版本..."
	@go mod tidy
	@GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(PROJECT_NAME)-windows-amd64.exe src/main/main.go
	@echo "构建完成: bin/$(PROJECT_NAME)-windows-amd64.exe"

# 构建 macOS 版本
build-macos:
	@echo "构建 macOS 版本..."
	@go mod tidy
	@GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(PROJECT_NAME)-macos-amd64 src/main/main.go
	@GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(PROJECT_NAME)-macos-arm64 src/main/main.go
	@echo "构建完成: bin/$(PROJECT_NAME)-macos-amd64, bin/$(PROJECT_NAME)-macos-arm64"

# 构建所有平台
build-all: build-linux build-windows build-macos
	@echo "所有平台构建完成!"

# 清理构建文件
clean:
	@echo "清理构建文件..."
	@rm -f bin/tradingbot bin/bollinger-trading
	@rm -f bin/*-linux-* bin/*-windows-* bin/*-macos-*
	@rm -rf backtest_results/
	@rm -rf logs/
	@rm -f config.json config.json.default
	@echo "清理完成"

# 运行测试
test:
	@echo "运行测试..."
	@go test -v ./src/...
	@echo "测试完成"

# 格式化代码
fmt:
	@echo "格式化代码..."
	@go fmt ./src/...
	@echo "代码格式化完成"

# 代码静态检查
lint:
	@echo "代码静态检查..."
	@go vet ./src/...
	@echo "静态检查完成"

# 更新依赖
deps:
	@echo "更新依赖..."
	@go mod tidy
	@go mod download
	@echo "依赖更新完成"

# 创建必要的目录
dirs:
	@mkdir -p bin
	@mkdir -p logs
	@mkdir -p backtest_results

# 运行程序（需要先构建）
run: build dirs
	@echo "运行 $(PROJECT_NAME)..."
	@./bin/$(PROJECT_NAME) --help

# 测试连接
ping: build
	@echo "🌐 测试币安连接..."
	@./bin/$(PROJECT_NAME) ping -v

# 测试K线数据
kline: build
	@echo "📊 测试K线数据..."
	@./bin/$(PROJECT_NAME) kline -s BTCUSDT -i 1h -l 5 -v

# 数据库回测（使用数据库优化）
backtest-db: build
	@echo "📊 运行数据库优化回测..."
	@./bin/$(PROJECT_NAME) bollinger

# 运行回测示例
run-backtest: build dirs
	@echo "运行回测示例..."
	@if [ ! -f "trading_config.json" ]; then \
		echo "创建默认配置文件..."; \
		./bin/$(PROJECT_NAME) bollinger --create-config -c trading_config.json; \
	fi
	@echo "开始回测..."
	@./bin/$(PROJECT_NAME) bollinger -c trading_config.json

# 安装到系统路径
install: build
	@echo "安装 $(PROJECT_NAME) 到系统路径..."
	@sudo cp bin/$(PROJECT_NAME) /usr/local/bin/
	@echo "安装完成，可使用 '$(PROJECT_NAME)' 命令"

# 创建发布包
release: build-all
	@echo "创建发布包..."
	@mkdir -p release
	@cp bin/$(PROJECT_NAME)-linux-amd64 release/
	@cp bin/$(PROJECT_NAME)-windows-amd64.exe release/
	@cp bin/$(PROJECT_NAME)-macos-amd64 release/
	@cp bin/$(PROJECT_NAME)-macos-arm64 release/
	@cp TRADING_README.md release/README.md
	@cp USAGE_GUIDE.md release/
	@cp example_config.json release/
	@tar -czf release/$(PROJECT_NAME)-$(VERSION).tar.gz -C release .
	@echo "发布包创建完成: release/$(PROJECT_NAME)-$(VERSION).tar.gz"

# 开发环境设置
dev-setup:
	@echo "设置开发环境..."
	@go mod tidy
	@go mod download
	@$(MAKE) dirs
	@echo "开发环境设置完成"

# 快速测试构建
quick-test: fmt lint
	@echo "快速测试构建..."
	@go build -o /tmp/$(PROJECT_NAME)-test src/main/main.go
	@rm -f /tmp/$(PROJECT_NAME)-test
	@echo "快速测试完成"