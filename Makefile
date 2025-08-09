.PHONY: build clean test install deps fmt lint vet check coverage release help \
        test-version test-add test-del test-check docker-build cross-compile

# Build variables
BINARY_NAME=cni-basic
BUILD_DIR=bin
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE)"
BUILD_FLAGS=-v $(LDFLAGS)

# Cross-compilation targets
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | sort

## deps: Download and verify dependencies
deps:
	$(GOMOD) download
	$(GOMOD) verify
	$(GOMOD) tidy

## build: Build the binary
build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

## build-debug: Build with debug symbols
build-debug:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)-debug .

## clean: Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	$(GOCLEAN)
	rm -f coverage.out coverage.html

## fmt: Format Go code
fmt:
	$(GOFMT) -s -w .

## lint: Run linter (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "Please install golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run

## vet: Run go vet
vet:
	$(GOCMD) vet ./...

## check: Run all checks (fmt, vet, lint)
check: fmt vet lint

## test: Run tests
test:
	$(GOTEST) -v -race ./...

## test-coverage: Run tests with coverage
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## bench: Run benchmarks
bench:
	$(GOTEST) -bench=. -benchmem ./...

## install: Install binary to system
install: build
	@echo "Installing $(BINARY_NAME) to /opt/cni/bin/"
	sudo mkdir -p /opt/cni/bin
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /opt/cni/bin/
	sudo chmod +x /opt/cni/bin/$(BINARY_NAME)

## uninstall: Remove binary from system
uninstall:
	sudo rm -f /opt/cni/bin/$(BINARY_NAME)

## cross-compile: Build for multiple platforms
cross-compile: clean
	@echo "Cross-compiling for multiple platforms..."
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-$${platform%/*}-$${platform#*/} . ; \
		echo "Built $(BUILD_DIR)/$(BINARY_NAME)-$${platform%/*}-$${platform#*/}" ; \
	done

## release: Create release binaries
release: clean check test cross-compile
	@echo "Creating release package..."
	mkdir -p $(BUILD_DIR)/release
	cd $(BUILD_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	cd $(BUILD_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	cd $(BUILD_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	cd $(BUILD_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	@echo "Release packages created in $(BUILD_DIR)/release/"

## docker-build: Build Docker image for testing
docker-build:
	docker build -t $(BINARY_NAME):$(VERSION) .

# Manual testing targets
## test-version: Test VERSION command
test-version: build
	@echo "Testing VERSION command..."
	echo '{}' | CNI_COMMAND=VERSION ./$(BUILD_DIR)/$(BINARY_NAME)

## test-add: Test ADD command
test-add: build
	@echo "Testing ADD command..."
	echo '{"cniVersion":"1.1.0","name":"test-network","type":"cni-basic","bridge":"cni0","subnet":"10.0.0.0/24","gateway":"10.0.0.1"}' | \
	CNI_COMMAND=ADD CNI_CONTAINERID=test123 CNI_NETNS=/tmp/test CNI_IFNAME=eth0 \
	./$(BUILD_DIR)/$(BINARY_NAME)

## test-del: Test DEL command
test-del: build
	@echo "Testing DEL command..."
	echo '{"cniVersion":"1.1.0","name":"test-network","type":"cni-basic"}' | \
	CNI_COMMAND=DEL CNI_CONTAINERID=test123 CNI_NETNS=/tmp/test CNI_IFNAME=eth0 \
	./$(BUILD_DIR)/$(BINARY_NAME)

## test-check: Test CHECK command
test-check: build
	@echo "Testing CHECK command..."
	echo '{"cniVersion":"1.1.0","name":"test-network","type":"cni-basic"}' | \
	CNI_COMMAND=CHECK CNI_CONTAINERID=test123 CNI_NETNS=/tmp/test CNI_IFNAME=eth0 \
	./$(BUILD_DIR)/$(BINARY_NAME)

## test-all-commands: Test all CNI commands
test-all-commands: test-version test-add test-del test-check
