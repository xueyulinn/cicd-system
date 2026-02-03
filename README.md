# CI/CD Pipeline Management Tool

A Go-based CLI tool for validating and managing CI/CD pipeline configurations. This tool parses YAML pipeline files and verifies their structure, dependencies, and validity.

## Overview

The e-team project is a CI/CD pipeline management system that provides:

- **Pipeline Validation**: Parse and validate YAML pipeline configurations
- **Dependency Checking**: Verify job dependencies and stage relationships
- **Error Reporting**: Detailed error messages with line and column information
- **CLI Interface**: Command-line tool for easy integration into development workflows

## Features

- Validate pipeline configuration files (YAML format)
- Check for circular dependencies between jobs
- Verify stage definitions and job assignments
- Support for complex dependency graphs
- Detailed error reporting with file locations
- Batch validation of multiple pipeline files
- Git repository validation

## Installation

### Prerequisites

- Go 1.25.6 or later
- Git repository (validation requires git repo)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/CS7580-SEA-SP26/e-team.git
cd e-team

#Windows: 
go build -o bin/cicd ./cicd
go install ./cicd

# macOS/Linux:
go build -o bin/cicd ./cicd
go install ./cicd

# If "cicd: command not found" on macOS, add Go bin to PATH:
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
source ~/.zshrc
```

The binary will be built as `bin/cicd` and can be installed to `$HOME/bin` by default.

## Usage

### Basic Commands

```bash
# Verify default pipeline file (.pipelines/pipeline.yaml)
cicd verify

# Verify specific pipeline file
cicd verify path/to/pipeline.yaml

# Verify all YAML files in a directory
cicd verify .pipelines/
```

### Available Commands

- `verify [config-file]` - Validate pipeline configuration files
- `help` - Show help information

## Pipeline Configuration Format

The tool expects YAML files with the following structure:

```yaml
# Pipeline metadata (optional)
pipeline:
  name: "Example Pipeline"

# Stage definitions
stages:
  - build
  - test
  - deploy

# Job definitions
job-name:
  - stage: build
  - image: golang:1.21
  - script:
    - "make build"

another-job:
  - stage: test
  - needs: [job-name]
  - image: golang:1.21
  - script:
    - "make test"
```

### Configuration Elements

- **pipeline**: Optional metadata with pipeline name
- **stages**: Array of stage names (must be defined before use)
- **jobs**: Job definitions with the following properties:
  - `stage`: Which stage the job belongs to (required)
  - `image`: Docker image to use (optional)
  - `script`: Commands to execute (optional)
  - `needs`: List of job dependencies (optional)

## Development

### Project Structure

```
e-team/
├── cicd/                 # CLI application
│   ├── cli/              # Command-line interface
│   │   ├── root.go       # Root command setup
│   │   └── verify.go     # Verify command implementation
│   └── main.go           # Application entry point
├── internal/             # Internal packages
│   ├── models/           # Data models and types
│   ├── parser/           # YAML parsing logic
│   └── verifier/         # Validation logic
├── .pipelines/           # Test pipeline configurations
├── dev-docs/             # Development documentation
└── Makefile             # Build automation
```


### Running Tests

```bash
# Run all tests
go test -v ./internal/...

# Run tests with coverage
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out -o coverage.html

# Open coverage report in browser
start coverage.html
```

## Validation Rules

The tool enforces the following validation rules:

1. **Git Repository**: Must be run within a Git repository
2. **Stage Definitions**: All stages must be defined before use
3. **Job References**: Jobs in `needs` must exist
4. **Circular Dependencies**: No circular dependencies allowed
5. **Stage Assignment**: Every job must be assigned to a valid stage
6. **YAML Syntax**: Files must be valid YAML

## Error Examples

### Invalid Configuration
```yaml
# Missing stage definition
stages: []
job-name:
  - stage: undefined_stage
```

**Error Output:**
```
.pipelines/pipeline.yaml:8:3: job 'job-name' references undefined stage 'undefined_stage'
```

### Circular Dependency
```yaml
stages: [build]
job-a:
  - stage: build
  - needs: [job-b]
job-b:
  - stage: build
  - needs: [job-a]
```

**Error Output:**
```
.pipelines/pipeline.yaml: circular dependency detected between jobs: job-a -> job-b -> job-a
```

## Technology Stack

- **Language**: Go 1.25.6
- **CLI Framework**: Cobra
- **YAML Parsing**: gopkg.in/yaml.v3
- **Testing**: Go standard testing package
- **Build Tool**: Make

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run `go test -coverprofile=coverage.out ./internal/...` to ensure coverage
6. Submit a pull request

## License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

## Team

This project is developed by the e-team for CS7580 SEA-SP26.
