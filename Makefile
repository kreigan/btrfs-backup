.PHONY: build test clean run install

# Build the binary
build:
	go build -o btrfs-backup ./cmd/btrfs-backup

# Run all tests with verbose output
test:
	go test -v ./...

# Clean up built artifacts
clean:
	rm -f btrfs-backup

# Build and run the program
run: build
	./btrfs-backup

# Install the binary to $GOPATH/bin
install:
	go install ./cmd/btrfs-backup