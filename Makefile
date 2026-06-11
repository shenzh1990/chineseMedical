.PHONY: run build test tidy

run:
	go run ./cmd/server

build:
	go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy
