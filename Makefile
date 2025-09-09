.PHONY: build test clean run install lint

# Build the binary with version info
build:
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	echo "Building version: $$VERSION"; \
	go build -ldflags "-X btrfs-backup/internal/cli.version=$$VERSION" -o btrfs-backup ./cmd/btrfs-backup

# Run linting checks
lint:
	go vet ./...
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "Code formatting issues found:"; \
		gofmt -s -l .; \
		exit 1; \
	fi
	golangci-lint run ./...

# Run all tests with verbose output
test: lint
	go test -v ./...

# Clean up built artifacts
clean:
	rm -f btrfs-backup

# Build and run the program
run: build
	./btrfs-backup

# Install the binary to $GOPATH/bin
install:
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	go install -ldflags "-X btrfs-backup/internal/cli.version=$$VERSION" ./cmd/btrfs-backup