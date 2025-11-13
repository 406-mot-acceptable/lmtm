.PHONY: build run clean install deps test

# Binary name
BINARY_NAME=tunneler
BUILD_DIR=./bin

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) cmd/tunneler/main.go
	@echo "✓ Built: $(BUILD_DIR)/$(BINARY_NAME)"

# Build with optimizations (smaller binary)
release:
	@echo "Building release version..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) cmd/tunneler/main.go
	@echo "✓ Built release: $(BUILD_DIR)/$(BINARY_NAME)"

# Run the application
run: build
	@$(BUILD_DIR)/$(BINARY_NAME)

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go get github.com/spf13/cobra@latest
	@go get github.com/charmbracelet/bubbletea@latest
	@go get github.com/charmbracelet/bubbles@latest
	@go get github.com/charmbracelet/lipgloss@latest
	@go get golang.org/x/crypto/ssh@latest
	@go get golang.org/x/term@latest
	@go get gopkg.in/yaml.v3@latest
	@go mod tidy
	@echo "✓ Dependencies installed"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean
	@echo "✓ Cleaned"

# Install to system
install: release
	@echo "Installing to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "✓ Installed to /usr/local/bin/$(BINARY_NAME)"

# Run tests
test:
	@go test -v ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  make build    - Build the application"
	@echo "  make release  - Build optimized release version"
	@echo "  make run      - Build and run the application"
	@echo "  make deps     - Install dependencies"
	@echo "  make clean    - Clean build artifacts"
	@echo "  make install  - Install to /usr/local/bin"
	@echo "  make test     - Run tests"
