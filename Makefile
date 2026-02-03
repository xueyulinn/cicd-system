.PHONY: build test test-coverage clean install run

BINARY_NAME=cicd
BUILD_DIR := bin

# Detect OS
ifeq ($(OS),Windows_NT)
	BINARY_EXT=.exe
	COPY=copy /Y
	define MKDIR_P
if not exist "$(1)" mkdir "$(1)"
endef
	define RM_FILE
if exist "$(1)" del /Q "$(1)"
endef
	define RMDIR_R
if exist "$(1)" rmdir /S /Q "$(1)"
endef
else
	BINARY_EXT=
	COPY=cp
	define MKDIR_P
mkdir -p "$(1)"
endef
	define RM_FILE
rm -f "$(1)"
endef
	define RMDIR_R
rm -rf "$(1)"
endef
endif

# Install location (override with PREFIX=...)
PREFIX ?= $(HOME)
BINDIR := $(PREFIX)/bin

build:
	$(call MKDIR_P,$(BUILD_DIR))
	go build -o $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT) ./cicd

test:
	go test -v ./internal/...

test-coverage:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean:
	$(call RMDIR_R,$(BUILD_DIR))
	$(call RM_FILE,coverage.out)
	$(call RM_FILE,coverage.html)

install: build
	$(call MKDIR_P,$(BINDIR))
	$(COPY) "$(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT)" "$(BINDIR)/"

run:
	go run ./cicd verify

deps:
	go mod download
	go mod tidy
