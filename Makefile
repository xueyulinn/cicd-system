.PHONY: build test test-coverage clean install run

BINARY_NAME=cicd
BUILD_DIR := bin

# Install location (override with PREFIX=...)
PREFIX ?= $(HOME)
BINDIR := $(PREFIX)/bin

build:
	mkdir -p bin
	go build -o bin/$(BINARY_NAME) ./cicd
	chmod +x bin/$(BINARY_NAME)

test:
	go test -v ./internal/...

test-coverage:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

install: build
# 	cp bin/$(BINARY_NAME) $(GOPATH)/bin/
	mkdir -p $(BINDIR)
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(BINDIR)/$(BINARY_NAME)

run:
	go run ./cicd verify

deps:
	go mod download
	go mod tidy