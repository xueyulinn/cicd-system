.PHONY: build test test-coverage clean install run

BINARY_NAME=cicd
BUILD_DIR := bin

# Detect OS
ifeq ($(OS),Windows_NT)
	BINARY_EXT=.exe
	PATHSEP=\\
	COPY=copy /Y
define MKDIR_P
	if not exist $(1) mkdir $(1)
endef
define RM_FILE
	if exist $(1) del /Q $(1)
endef
define RMDIR_R
	if exist $(1) rmdir /S /Q $(1)
endef
else
	BINARY_EXT=
	PATHSEP=/
	COPY=cp
define MKDIR_P
	mkdir -p $(1)
endef
define RM_FILE
	rm -f $(1)
endef
define RMDIR_R
	rm -rf $(1)
endef
endif

# Install location (override with PREFIX=...)
PREFIX ?= $(HOME)
BINDIR := $(PREFIX)$(PATHSEP)bin

build:
	$(call MKDIR_P,$(BUILD_DIR))
	go build -o $(BUILD_DIR)$(PATHSEP)$(BINARY_NAME)$(BINARY_EXT) ./cmd/cicd
ifneq ($(OS),Windows_NT)
	chmod +x $(BUILD_DIR)/$(BINARY_NAME)
endif

test:
	set CICD_TEST_MODE=1 && go test -v ./internal/... ./cmd/...

integration:
	go test -v ./internal/... ./cmd/...

test-coverage:
	go test -coverprofile=coverage.out ./internal/... ./cmd/...
	go tool cover -html=coverage.out -o coverage.html
	@echo Coverage report: coverage.html

clean:
	$(call RMDIR_R,$(BUILD_DIR))
	$(call RM_FILE,coverage.out)
	$(call RM_FILE,coverage.html)

install: build
	$(call MKDIR_P,$(BINDIR))
	$(COPY) $(BUILD_DIR)$(PATHSEP)$(BINARY_NAME)$(BINARY_EXT) $(BINDIR)$(PATHSEP)

run:
	go run ./cmd/cicd verify

deps:
	go mod download
	go mod tidy