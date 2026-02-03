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

PREFIX ?= $(HOME)
BINDIR := $(PREFIX)/bin

build:
    $(MKDIR) $(BUILD_DIR)
    go build -o $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT) ./cicd

test:
    go test -v ./internal/...

test-coverage:
    go test -coverprofile=coverage.out ./internal/...
    go tool cover -html=coverage.out -o coverage.html
    @echo Coverage report: coverage.html

clean:
    $(RMDIR) $(BUILD_DIR)
    $(RM) coverage.out coverage.html

install: build
    $(MKDIR) $(BINDIR)
    $(COPY) $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT) $(BINDIR)/

run:
    go run ./cicd verify

deps:
    go mod download
    go mod tidy
