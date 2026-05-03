# CLI Reference

This document summarizes the `cicd` command-line interface. The same commands and flags are available via `cicd --help` and the relevant subcommand help text.

## Root command

```text
cicd
```

Purpose:

- top-level entry point for pipeline validation, planning, execution, and reporting

Subcommands:

- `verify [config-file]`
- `dryrun [config-file]`
- `run`
- `report`
- `help`

## `verify [config-file]`

Purpose:

- validate one pipeline YAML file or every YAML file in a directory

Behavior:

- defaults to `.pipelines/pipeline.yaml` when no path is provided
- when a directory is passed, walks the directory and validates all `.yaml` / `.yml` files
- prints validation errors and exits non-zero on failure

Examples:

```bash
cicd verify
cicd verify .pipelines/pipeline.yaml
cicd verify .pipelines/
```

## `dryrun [config-file]`

Purpose:

- validate a pipeline and print the execution order without running jobs

Options:

- `-f, --format`
  - output format
  - supported values: `yaml`, `json`
  - default: `yaml`

Behavior:

- defaults to `.pipelines/pipeline.yaml` when no path is provided
- runs validation before generating the execution plan

Examples:

```bash
cicd dryrun
cicd dryrun .pipelines/pipeline.yaml
cicd dryrun .pipelines/pipeline.yaml --format json
```

## `run`

Purpose:

- execute a pipeline through the API gateway / execution service path

Options:

- `--file`
  - path to a pipeline YAML file
- `--name`
  - logical pipeline name to search for under `.pipelines/`
- `--branch`
  - resolve the run commit from the selected branch tip
- `--commit`
  - run the exact commit directly

Rules:

- exactly one of `--file` or `--name` is required
- `--branch` and `--commit` are mutually exclusive
- if neither `--branch` nor `--commit` is provided, the CLI uses the current `HEAD` commit
- if `--branch` is provided, the CLI resolves the branch tip to a commit before calling `/run`

Examples:

```bash
cicd run --file .pipelines/pipeline.yaml
cicd run --name "Default Pipeline"
cicd run --file .pipelines/pipeline.yaml --branch main
cicd run --file .pipelines/pipeline.yaml --commit HEAD
```

## `report`

Purpose:

- fetch historical pipeline execution data from the reporting path

Options:

- `--pipeline`
  - required pipeline name
- `--run`
  - optional run number for a specific run
- `--stage`
  - optional stage filter; requires `--run`
- `--job`
  - optional job filter; requires both `--run` and `--stage`
- `-f, --format`
  - output format
  - supported values: `yaml`, `json`
  - default: `yaml`

Examples:

```bash
cicd report --pipeline "Default Pipeline"
cicd report --pipeline "Default Pipeline" --run 1
cicd report --pipeline "Default Pipeline" --run 1 --stage build
cicd report --pipeline "Default Pipeline" --run 1 --stage test --job unit-tests
cicd report --pipeline "Default Pipeline" --run 1 --format json
```

## Environment variables

- `GATEWAY_URL`
  - base URL used by the CLI to contact the API gateway
  - default local value is `http://localhost:8000`
- `CICD_TEST_MODE`
  - when set to `1`, selected CLI commands use direct in-process logic for tests instead of going through the gateway

## Help commands

These commands should always be available for evaluator verification:

```bash
cicd --help
cicd verify --help
cicd dryrun --help
cicd run --help
cicd report --help
```
