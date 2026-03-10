# Developer Installation and Usage

## Purpose

This document describes how developers can build, run, and test the project from source on a fresh Ubuntu 24.04.4 LTS machine.

## Prerequisites

- **Git** (pre-installed on Ubuntu 24.04)
- **Go 1.25.6** or later
- **Docker and Docker Compose**
- **Make**

---

## Step 1: Install Dependencies

### Install Docker

If Docker is not installed, follow the instructions in [non-dev-install.md](non-dev-install.md#step-1-install-docker), or run:

```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y ca-certificates curl

sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
  -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
  https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io \
  docker-buildx-plugin docker-compose-plugin

sudo usermod -aG docker $USER
```

Log out and log back in (or run `newgrp docker`) for the group change to take effect.

### Install Go

```bash
sudo apt install -y wget
wget https://go.dev/dl/go1.25.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.6.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

Verify:

```bash
go version
```

### Install Make and Build Tools

```bash
sudo apt install -y make build-essential
```

---

## Step 2: Get the Source Code

```bash
git clone https://github.com/CS7580-SEA-SP26/e-team.git
cd e-team
```

---

## Step 3: Build the CLI

```bash
make build
```

This creates the CLI binary at `bin/cicd`.

To install it system-wide:

```bash
make install
```

Verify:

```bash
./bin/cicd --help
```

---

## Step 4: Start Backend Services

Build and start all services from source:

```bash
docker compose up -d --build
```

This builds and starts:

- PostgreSQL
- API Gateway
- Validation Service
- Execution Service
- Worker Service
- Reporting Service
- Database migration service

The first build may take several minutes as it downloads Go dependencies and compiles each service.

---

## Step 5: Verify the Services

Check container status:

```bash
docker compose ps
```

All services should show `Up` status. Then check the gateway health:

```bash
curl http://localhost:8000/health
```

You should see a response indicating all services are healthy.

---

## Step 6: Run the CLI

Validate a pipeline:

```bash
./bin/cicd verify .pipelines/pipeline.yaml
```

Show execution plan:

```bash
./bin/cicd dryrun .pipelines/pipeline.yaml
```

Run a pipeline:

```bash
./bin/cicd run --file .pipelines/pipeline.yaml
```

For example pipelines (success, failure, validation error, reports), see the [README](README.md).

---

## Step 7: Run Tests

```bash
go test -v ./internal/... ./cmd/...
```

---

## Stop the Services

```bash
docker compose down
```

To remove all data (database volumes, etc.):

```bash
docker compose down -v
```

To rebuild a single service after code changes:

```bash
docker compose up -d --build <service-name>
```

---

## Troubleshooting

| Problem                                             | Solution                                                      |
| --------------------------------------------------- | ------------------------------------------------------------- |
| `permission denied` when running `docker`           | Log out and log back in, or run `newgrp docker`               |
| `make: command not found`                           | Run: `sudo apt install -y make build-essential`               |
| `go: command not found`                             | Make sure Go is installed and PATH is set: `source ~/.bashrc` |
| Services exit immediately after `docker compose up` | Check logs: `docker compose logs <service-name>`              |
| CLI cannot connect to backend                       | Ensure services are running: `docker compose ps`              |
