.PHONY: build test test-coverage clean install run

BINARY_NAME=cicd

build:
	mkdir -p bin
	go build -o bin/$(BINARY_NAME) ./cmd/cli
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
	cp bin/$(BINARY_NAME) $(GOPATH)/bin/

run:
	go run ./cmd/cli verify

deps:
	go mod download
	go mod tidy