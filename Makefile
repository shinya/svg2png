.PHONY: build clean test example install

# ビルド設定
BINARY_NAME=svgpng
BUILD_DIR=build
MAIN_PATH=cmd/svgpng/main.go

# デフォルトターゲット
all: build

# バイナリのビルド
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

# リリースビルド
release: clean
	@echo "Building release version..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "Release builds completed"

# クリーンアップ
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f test_output.png
	@echo "Clean completed"

# テストの実行
test:
	@echo "Running tests..."
	go test ./...
	@echo "Tests completed"

# 例の実行
example: build
	@echo "Running example conversion..."
	./$(BUILD_DIR)/$(BINARY_NAME) -in examples/simple.svg -out example_output.png -w 800 -h 600 -bg white
	@echo "Example completed: example_output.png"

# インストール
install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation completed"

# 依存関係の整理
deps:
	@echo "Tidying dependencies..."
	go mod tidy
	@echo "Dependencies tidied"

# コードのフォーマット
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Code formatted"

# リントチェック
lint:
	@echo "Running linter..."
	golangci-lint run
	@echo "Lint check completed"

# ヘルプ
help:
	@echo "Available targets:"
	@echo "  build     - Build the binary"
	@echo "  release   - Build release versions for multiple platforms"
	@echo "  clean     - Clean build artifacts"
	@echo "  test      - Run tests"
	@echo "  example   - Run example conversion"
	@echo "  install   - Install binary to /usr/local/bin"
	@echo "  deps      - Tidy dependencies"
	@echo "  fmt       - Format code"
	@echo "  lint      - Run linter"
	@echo "  help      - Show this help"
