.PHONY: build test test-coverage clean install run

BINARY_NAME=cicd
BUILD_DIR := bin

# Detect OS
ifeq ($(OS),Windows_NT)
    BINARY_EXT=.exe
    RM=del /Q
    RMDIR=rmdir /S /Q
    MKDIR=if not exist $(BUILD_DIR) mkdir
    COPY=copy
else
    BINARY_EXT=
    RM=rm -f
    RMDIR=rm -rf
    MKDIR=mkdir -p
    COPY=cp
endif

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
	mkdir -p $(BINDIR)
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(BINDIR)/$(BINARY_NAME)

run:
	go run ./cicd verify

deps:
	go mod download
	go mod tidy