# Go Development Tools and Configuration

## Essential Tools

### Go Commands
```bash
# Build
go build              # Build current package
go build -o myapp     # Build with custom output name
go build ./cmd/...    # Build all commands

# Run
go run main.go        # Run single file
go run ./cmd/myapp    # Run package

# Test
go test               # Test current package
go test ./...         # Test all packages
go test -v            # Verbose output
go test -cover        # Show coverage
go test -race         # Race detector
go test -bench .      # Run benchmarks
go test -short        # Run short tests only

# Module management
go mod init github.com/user/project
go mod tidy           # Add missing, remove unused
go mod download       # Download dependencies
go mod vendor         # Copy dependencies to vendor/
go mod verify         # Verify dependencies

# Get packages
go get github.com/pkg/errors
go get -u             # Update dependencies
go get -u ./...       # Update all dependencies

# Generate
go generate ./...     # Run code generators

# Format
go fmt ./...          # Format all packages
gofmt -s -w .         # Simplify and write

# Vet
go vet ./...          # Examine code for issues

# Install
go install            # Install current package
go install golang.org/x/tools/cmd/goimports@latest
```

### golangci-lint
Comprehensive linter aggregator.

**.golangci.yml configuration:**
```yaml
run:
  timeout: 5m
  tests: true
  skip-dirs:
    - vendor
    - third_party

linters:
  enable:
    - errcheck       # Check error handling
    - gosimple       # Simplify code
    - govet          # Go vet
    - ineffassign    # Detect ineffectual assignments
    - staticcheck    # Advanced static analysis
    - typecheck      # Type checking
    - unused         # Check for unused code
    - gofmt          # Format checking
    - goimports      # Import formatting
    - misspell       # Spell checking
    - goconst        # Find repeated strings
    - gocritic       # Comprehensive checks
    - gocyclo        # Cyclomatic complexity
    - revive         # Fast, configurable linter
    - gosec          # Security issues
    - bodyclose      # HTTP response body closed
    - noctx          # HTTP request without context
    - unparam        # Unused function parameters
    - dupl           # Duplicate code detection
    - exhaustive     # Check exhaustiveness

linters-settings:
  errcheck:
    check-type-assertions: true
    check-blank: true
  
  govet:
    check-shadowing: true
  
  gocyclo:
    min-complexity: 15
  
  dupl:
    threshold: 100
  
  goconst:
    min-len: 3
    min-occurrences: 3
  
  misspell:
    locale: US
  
  revive:
    rules:
      - name: blank-imports
      - name: context-as-argument
      - name: context-keys-type
      - name: dot-imports
      - name: error-return
      - name: error-strings
      - name: error-naming
      - name: exported
      - name: if-return
      - name: increment-decrement
      - name: var-naming
      - name: var-declaration
      - name: package-comments
      - name: range
      - name: receiver-naming
      - name: time-naming
      - name: unexported-return
      - name: indent-error-flow
      - name: errorf
  
  staticcheck:
    checks: ["all"]

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
  
  exclude-rules:
    - path: _test\.go
      linters:
        - dupl
        - gosec
        - goconst

output:
  format: colored-line-number
  print-issued-lines: true
  print-linter-name: true
```

**Usage:**
```bash
# Run all enabled linters
golangci-lint run

# Run specific linters
golangci-lint run --enable=gosec,errcheck

# Auto-fix issues
golangci-lint run --fix

# Fast mode (cache results)
golangci-lint run --fast
```

### goimports
Enhanced gofmt with import management.

```bash
# Install
go install golang.org/x/tools/cmd/goimports@latest

# Format and fix imports
goimports -w .

# Format with local import prefix
goimports -local github.com/myorg -w .
```

### gopls
Official Go language server for IDEs.

```bash
# Install
go install golang.org/x/tools/gopls@latest
```

**VS Code settings.json:**
```json
{
  "go.useLanguageServer": true,
  "gopls": {
    "analyses": {
      "unusedparams": true,
      "shadow": true
    },
    "staticcheck": true,
    "usePlaceholders": true
  }
}
```

### go-critic
Opinionated Go linter.

```bash
# Install
go install github.com/go-critic/go-critic/cmd/gocritic@latest

# Run
gocritic check ./...

# With specific checks
gocritic check -enable='#diagnostic,#style' ./...
```

## Testing Tools

### testify
Popular assertion and mocking library.

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"
)

func TestSomething(t *testing.T) {
    // Assertions (continues on failure)
    assert.Equal(t, expected, actual)
    assert.NotNil(t, obj)
    assert.Error(t, err)
    
    // Requirements (stops on failure)
    require.NoError(t, err)
    require.NotEmpty(t, list)
}

// Mock example
type MockDB struct {
    mock.Mock
}

func (m *MockDB) Get(id string) (User, error) {
    args := m.Called(id)
    return args.Get(0).(User), args.Error(1)
}

func TestWithMock(t *testing.T) {
    mockDB := new(MockDB)
    mockDB.On("Get", "123").Return(User{ID: "123"}, nil)
    
    // Use mock
    user, err := mockDB.Get("123")
    
    mockDB.AssertExpectations(t)
}
```

### go-cmp
Google's package for comparing Go values.

```go
import "github.com/google/go-cmp/cmp"

if diff := cmp.Diff(want, got); diff != "" {
    t.Errorf("mismatch (-want +got):\n%s", diff)
}

// Custom comparers
if diff := cmp.Diff(want, got, 
    cmpopts.IgnoreFields(User{}, "UpdatedAt"),
); diff != "" {
    t.Errorf("mismatch:\n%s", diff)
}
```

### gotests
Generate table-driven tests automatically.

```bash
# Install
go install github.com/cweill/gotests/gotests@latest

# Generate tests
gotests -all -w file.go

# Generate tests for specific function
gotests -only FunctionName -w file.go
```

## Code Quality Tools

### staticcheck
Advanced static analysis.

```bash
# Install
go install honnef.co/go/tools/cmd/staticcheck@latest

# Run
staticcheck ./...
```

### errcheck
Check that errors are handled.

```bash
# Install
go install github.com/kisielk/errcheck@latest

# Run
errcheck ./...

# Ignore specific errors
errcheck -ignore 'Close' ./...
```

### gosec
Security scanner.

```bash
# Install
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Run
gosec ./...

# Generate report
gosec -fmt=json -out=results.json ./...
```

## Build and Deployment

### Makefile Example
```makefile
.PHONY: build test lint clean install

BINARY_NAME=myapp
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.Version=${VERSION}"

all: test build

build:
	go build ${LDFLAGS} -o ${BINARY_NAME} ./cmd/myapp

test:
	go test -v -race -cover ./...

lint:
	golangci-lint run

bench:
	go test -bench=. -benchmem ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

clean:
	go clean
	rm -f ${BINARY_NAME}
	rm -f coverage.out

install:
	go install ${LDFLAGS} ./cmd/myapp

deps:
	go mod download
	go mod verify

update:
	go get -u ./...
	go mod tidy

docker:
	docker build -t ${BINARY_NAME}:${VERSION} .

run:
	go run ./cmd/myapp
```

### Dockerfile Example
```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o myapp ./cmd/myapp

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/myapp .

EXPOSE 8080

CMD ["./myapp"]
```

### Build Tags
```go
// +build integration

package mypackage

// This file only included when: go test -tags=integration
```

## Editor Configuration

### VS Code settings.json
```json
{
  "go.toolsManagement.autoUpdate": true,
  "go.formatTool": "goimports",
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "package",
  "go.vetOnSave": "package",
  "go.buildOnSave": "package",
  "go.testOnSave": false,
  "go.coverOnSave": false,
  "go.useLanguageServer": true,
  "gopls": {
    "analyses": {
      "unusedparams": true,
      "shadow": true,
      "nilness": true
    },
    "staticcheck": true,
    "usePlaceholders": true,
    "completeUnimported": true,
    "deepCompletion": true
  },
  "[go]": {
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
      "source.organizeImports": true
    }
  }
}
```

### .editorconfig
```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true

[*.go]
indent_style = tab
indent_size = 4

[*.{yml,yaml}]
indent_style = space
indent_size = 2

[Makefile]
indent_style = tab
```

## CI/CD Configuration

### GitHub Actions Example
```yaml
name: Go CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Cache Go modules
      uses: actions/cache@v3
      with:
        path: ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
        restore-keys: |
          ${{ runner.os }}-go-
    
    - name: Download dependencies
      run: go mod download
    
    - name: Run tests
      run: go test -v -race -coverprofile=coverage.out ./...
    
    - name: Upload coverage
      uses: codecov/codecov-action@v3
      with:
        files: ./coverage.out
  
  lint:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: golangci-lint
      uses: golangci/golangci-lint-action@v3
      with:
        version: latest
  
  build:
    runs-on: ubuntu-latest
    needs: [test, lint]
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Build
      run: go build -v ./...
```

## Performance Profiling

### CPU Profiling
```bash
# Run with CPU profiling
go test -cpuprofile cpu.prof -bench .

# Analyze
go tool pprof cpu.prof
```

### Memory Profiling
```bash
# Run with memory profiling
go test -memprofile mem.prof -bench .

# Analyze
go tool pprof mem.prof
```

### HTTP Profiling
```go
import _ "net/http/pprof"

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    
    // Your application code
}
```

Access profiles at:
- http://localhost:6060/debug/pprof/
- http://localhost:6060/debug/pprof/heap
- http://localhost:6060/debug/pprof/goroutine

### Trace
```bash
# Generate trace
go test -trace trace.out

# View trace
go tool trace trace.out
```
