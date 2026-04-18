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

If Docker is not installed, follow the instructions in [install_non-developers.md](install_non-developers.md#step-1-install-docker), or run:

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
git clone https://github.com/xueyulinn/cicd-system.git
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

Start the local infrastructure first:

```bash
docker compose --env-file compose.values.env up -d postgres rabbitmq db-migrate
```

This starts:

- PostgreSQL
- RabbitMQ
- Database migration service

Then start the Go services from source:

```bash
./scripts/start-services.sh
```

This starts:

- API Gateway
- Validation Service
- Execution Service
- Worker Service
- Reporting Service

If you prefer to run everything in containers instead of host Go processes, use:

```bash
docker compose --env-file compose.values.env up -d --build
```

The first build may take several minutes as it downloads Go dependencies and compiles each service.

---

## Step 5: Verify the Services

Check container status:

```bash
docker compose --env-file compose.values.env ps
```

PostgreSQL and RabbitMQ should show `Up` status. Then check the service endpoints:

```bash
curl http://localhost:8000/health
curl http://localhost:8001/ready
curl http://localhost:8004/ready
```

You should see successful responses from the gateway, validation service, and reporting service. If `reporting-service` cannot connect to Postgres on host port `5432`, move the local Postgres host port to a free port such as `55432` and update `DATABASE_URL` / `REPORT_DB_URL` accordingly.

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

For example pipelines (success, failure, validation error, reports), see the [README](../../README.md).

---

## Step 7: Run Tests

```bash
go test -v ./internal/... ./cmd/...
```

---

## Stop the Services

```bash
docker compose --env-file compose.values.env down
```

To remove all data (database volumes, etc.):

```bash
docker compose --env-file compose.values.env down -v
```

To rebuild a single service after code changes:

```bash
docker compose --env-file compose.values.env up -d --build <service-name>
```

---

## Troubleshooting

| Problem                                             | Solution                                                      |
| --------------------------------------------------- | ------------------------------------------------------------- |
| `permission denied` when running `docker`           | Log out and log back in, or run `newgrp docker`               |
| `make: command not found`                           | Run: `sudo apt install -y make build-essential`               |
| `go: command not found`                             | Make sure Go is installed and PATH is set: `source ~/.bashrc` |
| Services exit immediately after startup             | Check logs: `docker compose --env-file compose.values.env logs <service-name>` |
| CLI cannot connect to backend                       | Ensure infrastructure is running: `docker compose --env-file compose.values.env ps` |
| Reporting service fails to connect to Postgres      | Check for a host port conflict on `5432`; if needed, remap Postgres to another host port such as `55432` and update `DATABASE_URL` / `REPORT_DB_URL` |
