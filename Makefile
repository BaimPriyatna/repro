.PHONY: all build test lint fmt vet clean tidy

BINARY   := repro
CMD_PATH := ./cmd/repro
OUT_DIR  := ./bin

## all: build the binary (default target)
all: build

## build: compile the CLI binary into ./bin/
build:
	@mkdir -p $(OUT_DIR)
	go build -o $(OUT_DIR)/$(BINARY) $(CMD_PATH)

## run: build and run the CLI
run: build
	$(OUT_DIR)/$(BINARY)

## test: run the full test suite with race detector
test:
	go test -race -count=1 ./...

## test-v: verbose test output
test-v:
	go test -race -count=1 -v ./...

## cover: run tests and open HTML coverage report
cover:
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

## lint: run golangci-lint (must be installed separately)
lint:
	golangci-lint run ./...

## fmt: format all Go source files
fmt:
	gofmt -w -s .

## vet: run go vet
vet:
	go vet ./...

## tidy: tidy and verify module dependencies
tidy:
	go mod tidy
	go mod verify

## clean: remove build artifacts
clean:
	rm -rf $(OUT_DIR) coverage.out

## help: print this help
help:
	@grep -E '^## ' Makefile | sed 's/## //'
