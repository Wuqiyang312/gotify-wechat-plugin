GOTIFY_VERSION=v2.9.0
GO_VERSION=1.25.1

PLUGIN_NAME=gotify-wechat-plugin
PLUGIN_ENTRY=.
MODULE_NAME=github.com/yourusername/gotify-wechat-plugin

BUILD_DIR=build

.PHONY: all
all: build-linux-amd64 build-linux-arm64 build-linux-arm-7

.PHONY: check-compat
check-compat:
	@echo "Checking dependency compatibility..."
	@go get -u github.com/gotify/plugin-api/cmd/gomod-cap
	@curl -sL https://raw.githubusercontent.com/gotify/server/$(GOTIFY_VERSION)/go.mod -o /tmp/gotify-go.mod
	@go run github.com/gotify/plugin-api/cmd/gomod-cap -from /tmp/gotify-go.mod -to go.mod
	@go mod tidy
	@rm /tmp/gotify-go.mod

.PHONY: build-linux-amd64
build-linux-amd64:
	@echo "Building for linux/amd64..."
	@mkdir -p $(BUILD_DIR)
	@docker run --rm \
		-v "$(PWD):/proj" \
		-w /proj \
		gotify/build:$(GO_VERSION)-linux-amd64 \
		go build -a -installsuffix cgo -ldflags "-w -s" \
		-buildmode=plugin \
		-o $(BUILD_DIR)/$(PLUGIN_NAME)-linux-amd64.so \
		$(PLUGIN_ENTRY)

.PHONY: build-linux-arm64
build-linux-arm64:
	@echo "Building for linux/arm64..."
	@mkdir -p $(BUILD_DIR)
	@docker run --rm \
		-v "$(PWD):/proj" \
		-w /proj \
		gotify/build:$(GO_VERSION)-linux-arm64 \
		go build -a -installsuffix cgo -ldflags "-w -s" \
		-buildmode=plugin \
		-o $(BUILD_DIR)/$(PLUGIN_NAME)-linux-arm64.so \
		$(PLUGIN_ENTRY)

.PHONY: build-linux-arm-7
build-linux-arm-7:
	@echo "Building for linux/arm-7..."
	@mkdir -p $(BUILD_DIR)
	@docker run --rm \
		-v "$(PWD):/proj" \
		-w /proj \
		gotify/build:$(GO_VERSION)-linux-arm-7 \
		go build -a -installsuffix cgo -ldflags "-w -s" \
		-buildmode=plugin \
		-o $(BUILD_DIR)/$(PLUGIN_NAME)-linux-arm-7.so \
		$(PLUGIN_ENTRY)

.PHONY: clean
clean:
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR)

.PHONY: test
test:
	@echo "Running tests..."
	@go test -v ./...

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

.PHONY: vet
vet:
	@echo "Running go vet..."
	@go vet ./...
