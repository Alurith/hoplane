# Show the available project recipes.
[group('Project')]
default:
    @just --list

# Format all Go packages.
[group('Quality')]
fmt:
    go fmt ./...

# Check that all Go files are formatted.
[group('Quality')]
fmt-check:
    test -z "$(gofmt -l cmd internal)"

# Run all tests.
[group('Quality')]
test:
    go test ./...

# Run static analysis.
[group('Quality')]
vet:
    go vet ./...

# Scan dependencies and reachable code for known vulnerabilities.
[group('Quality')]
vulncheck:
    go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...

# Run formatting checks, tests, and static analysis.
[group('Quality')]
check: fmt-check test vet

# Synchronize Go module dependencies.
[group('Dependencies')]
tidy:
    go mod tidy

# Build the hoplane binary.
[group('Project')]
build:
    mkdir -p bin
    go build -o bin/hoplane ./cmd/hoplane

# Install hoplane into GOPATH/bin.
[group('Project')]
install:
    go install ./cmd/hoplane

# Remove locally built binaries.
[group('Project')]
clean:
    rm -rf bin

# Run the CLI, forwarding all arguments.
[group('CLI')]
run *args:
    go run ./cmd/hoplane {{args}}
