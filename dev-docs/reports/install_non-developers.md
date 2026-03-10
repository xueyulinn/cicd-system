# Non-Developer Installation and Usage

## Purpose

This document describes how non-developers can download and run the application without building it from source.

## Prerequisites

- Docker and Docker Compose
- Git (required only to run pipelines inside a Git repository)

You do not need Go, Python, Java, or any other language runtime to use the released application.

## Download the CLI

Open the GitHub release page and download the binary that matches your operating system:

- `cicd-linux-amd64`
- `cicd-linux-arm64`
- `cicd-darwin-amd64`
- `cicd-darwin-arm64`

Release page:

- `https://github.com/CS7580-SEA-SP26/e-team/releases/tag/v0.1.0-beta`

After downloading, make the binary executable and move it to a directory in your `PATH` if needed.

Example on Linux:

```bash
chmod +x cicd-linux-amd64
mv cicd-linux-amd64 cicd
./cicd --help
```

## Download the System Components

From the same GitHub release page, download:

- `compose-bundle-v0.1.0-beta.tar.gz`

Extract it:

```bash
tar -xzf compose-bundle-v0.1.0-beta.tar.gz
cd compose-bundle
```

## Configure the Compose Bundle

Copy the environment template:

```bash
cp .env.example .env
```

You may edit `.env` if you want to change image tags or ports.

## Start the System

Run:

```bash
docker compose up -d
```

Check status:

```bash
docker compose ps
```

Check the API gateway:

```bash
curl http://localhost:8000/health
```

## Use the CLI

Run the CLI inside a Git repository that contains a pipeline file.

Validate:

```bash
./cicd verify .pipelines/pipeline.yaml
```

Dry-run:

```bash
./cicd dryrun .pipelines/pipeline.yaml
```

Run:

```bash
./cicd run --file .pipelines/pipeline.yaml
```

## Stop the System

```bash
docker compose down
```
