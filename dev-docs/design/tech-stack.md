# Tech Stack

## Overview
This document outlines the technology stack, tools, libraries, and frameworks used in the CI/CD system project.

---

## Components

### 1. CLI (Command Line Interface)

**Programming Language:**
- Go 1.21+

**Build Tools:**
- `go build` - Native Go build tool
- `make` - Makefile for task automation

**Build Tool Plugins/Tools:**
- **Test Coverage:** `go test -cover` with `go tool cover` for HTML reports
- **Documentation:** `godoc` or `pkgsite` for documentation generation
- **Static Analysis:**
    - `golangci-lint` (aggregates multiple linters)
    - `go vet` (built-in static analyzer)
    - `staticcheck`
- **Code Style/Linters:**
    - `gofmt` / `gofumpt` (code formatting)
    - `golangci-lint` (includes `gofmt`, `govet`, `errcheck`, etc.)

**Libraries:**
- `github.com/spf13/cobra` - CLI framework for building command-line applications
- `github.com/spf13/viper` - Configuration management
- `gopkg.in/yaml.v3` - YAML parsing for configuration files
- `github.com/fatih/color` - Colored terminal output
- Standard library: `net/http` - HTTP client for API calls

**Frameworks:**
- Cobra CLI framework

---

### 2. Coordinator (Control Server)

**Programming Language:**
- Go 1.21+

**Build Tools:**
- `go build`
- `make`

**Build Tool Plugins/Tools:**
- **Test Coverage:** `go test -cover` (minimum 70% coverage target)
- **Documentation:** `godoc` / `pkgsite`
- **Static Analysis:** `golangci-lint`, `go vet`, `staticcheck`
- **Code Style/Linters:** `gofmt`, `gofumpt`, `golangci-lint`

**Libraries:**
- `github.com/gin-gonic/gin` - HTTP web framework for REST API
- `gopkg.in/yaml.v3` - YAML parsing and validation
- `github.com/rabbitmq/amqp091-go` - RabbitMQ client
- `gorm.io/gorm` - ORM for database operations
- `gorm.io/driver/postgres` - PostgreSQL driver for GORM
- `github.com/google/uuid` - UUID generation for job IDs
- `github.com/go-playground/validator/v10` - Struct validation
- Standard library: `encoding/json`, `net/http`

**Frameworks:**
- Gin Web Framework

---

### 3. Runner (Job Executor)

**Programming Language:**
- Go 1.21+

**Build Tools:**
- `go build`
- `make`

**Build Tool Plugins/Tools:**
- **Test Coverage:** `go test -cover` (minimum 70% coverage target)
- **Documentation:** `godoc` / `pkgsite`
- **Static Analysis:** `golangci-lint`, `go vet`, `staticcheck`
- **Code Style/Linters:** `gofmt`, `gofumpt`, `golangci-lint`

**Libraries:**
- `github.com/docker/docker` - Docker SDK for container management
- `github.com/rabbitmq/amqp091-go` - RabbitMQ client for job queue
- `github.com/google/uuid` - Job ID handling
- Standard library: `os/exec`, `io`, `context`

**Frameworks:**
- None (uses standard library and Docker SDK)

---

### 4. Verifier, Loader, Scheduler

**Programming Language:**
- Go 1.21+

**Build Tools:**
- `go build`
- `make`

**Build Tool Plugins/Tools:**
- Same as Coordinator (integrated as packages)

**Libraries:**
- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/xeipuuv/gojsonschema` - JSON schema validation for YAML configs
- `github.com/go-playground/validator/v10` - Struct validation

**Frameworks:**
- None (utility packages integrated into Coordinator)

---

## Infrastructure Software

### Database
- **PostgreSQL 15+**
    - Primary datastore for job metadata, execution history, and results
    - Chosen for ACID compliance, reliability, and excellent Go support via GORM

### Message Queue
- **RabbitMQ 3.12+**
    - Message broker for asynchronous job distribution between Coordinator and Runners
    - Provides reliability, delivery guarantees, and decoupling

### Container Runtime
- **Docker 24+**
    - Required on Runner nodes for isolated job execution
    - Docker SDK used for programmatic container management

---

## Repository Configuration

### Repositories

1. **`cicd-system`** (Monorepo)
    - Contains all components: CLI, Coordinator, Runner, and shared libraries
    - Justification: Simplified dependency management and consistent versioning for a tightly coupled system

Alternative structure (if using separate repos):
- `cicd-cli` - Command-line interface
- `cicd-coordinator` - Coordinator server
- `cicd-runner` - Job runner
- `cicd-shared` - Shared libraries and utilities

---

## CI/CD Pipeline Configuration

### Pipeline 1: Pull Request (PR) Pipeline

**Triggers:** On pull request to `main` branch

**Jobs:**
1. **Build**
    - Compile all Go modules
    - Ensure no build errors

2. **Lint**
    - Run `golangci-lint` with all enabled linters
    - Run `gofmt` check (fail if code is not formatted)
    - Run `go vet` for suspicious constructs

3. **Test**
    - Run `go test ./...` for all packages
    - Generate test coverage report
    - **Minimum coverage requirement: 70%**
    - Fail PR if coverage drops below threshold

4. **Static Analysis**
    - Run `staticcheck` for code quality issues
    - Generate and upload static analysis report

5. **Security Scan**
    - Run `gosec` for security vulnerabilities
    - Check dependencies with `go list -m all` and vulnerability database

**Status:** Required checks must pass before merge

---

### Pipeline 2: Main Branch Pipeline

**Triggers:** On push/merge to `main` branch

**Jobs:**
1. **Build**
    - Compile all components
    - Build Docker images for Coordinator and Runner

2. **Test**
    - Run full test suite
    - Generate comprehensive coverage report (HTML + badge)
    - **Minimum coverage requirement: 70%**

3. **Integration Tests**
    - Spin up test environment (PostgreSQL, RabbitMQ via Docker Compose)
    - Run end-to-end integration tests
    - Verify CLI → Coordinator → Runner flow

4. **Documentation**
    - Generate `godoc` documentation
    - Update documentation site (if applicable)

5. **Docker Image Push**
    - Tag images with `latest` and commit SHA
    - Push to container registry (Docker Hub or GitHub Container Registry)

---

### Pipeline 3: Release Pipeline

**Triggers:** On tag push matching `v*.*.*` (e.g., `v1.0.0`)

**Jobs:**
1. **Build Release Artifacts**
    - Cross-compile binaries for multiple platforms:
        - `linux/amd64`
        - `linux/arm64`
        - `darwin/amd64` (macOS Intel)
        - `darwin/arm64` (macOS Apple Silicon)
        - `windows/amd64`

2. **Test**
    - Run full test suite with coverage
    - Run integration tests
    - **Minimum coverage requirement: 75% for releases**

3. **Security Audit**
    - Run comprehensive security scan
    - Check all dependencies for known vulnerabilities

4. **Docker Images**
    - Build production Docker images
    - Tag with version number and `latest`
    - Push to container registry

5. **GitHub Release**
    - Create GitHub release with changelog
    - Attach compiled binaries
    - Include checksums (SHA256)

6. **Documentation**
    - Generate and publish release documentation
    - Update version in documentation site

**Status:** Manual approval required before deployment to production

---

## Development Tools

### Version Control
- **Git** with GitHub for repository hosting
- **Branch Protection Rules:**
    - Require PR reviews (minimum 1 approval)
    - Require status checks to pass
    - Require branches to be up to date before merging

### Local Development
- **Docker Compose** - For running PostgreSQL and RabbitMQ locally
- **Make** - Task automation (build, test, lint, run)
- **Air** (optional) - Hot reload for Go applications during development

### Code Quality Thresholds
- **Test Coverage:** Minimum 70% (75% for releases)
- **Linter:** Zero warnings/errors from `golangci-lint`
- **Code Formatting:** All code must pass `gofmt`
- **Security:** Zero high/critical vulnerabilities from `gosec`

---

## Rationale

### Why Go?
- Excellent concurrency support (goroutines) for handling multiple jobs
- Fast compilation and execution
- Strong standard library for HTTP, networking, and system operations
- Native cross-platform compilation
- Great ecosystem for DevOps tools (Docker SDK, excellent CLI frameworks)
- Static binaries simplify deployment

### Why PostgreSQL?
- ACID compliance ensures data integrity
- Mature, reliable, and widely supported
- Excellent Go support via GORM
- Suitable for medium-load applications

### Why RabbitMQ?
- Industry-standard message broker
- Provides delivery guarantees and persistence
- Good Go client library
- Enables horizontal scaling of runners

### Why Gin Framework?
- High performance HTTP framework
- Clean API and middleware support
- Excellent documentation and community
- Built-in JSON validation and binding