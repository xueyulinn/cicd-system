# Non-Developer Installation and Usage

## Purpose

This document describes how to download, install, and run the CI/CD application without building anything from source. No programming knowledge is required.

## Prerequisites

- A machine running **Ubuntu 24.04.4 LTS** (x86_64 architecture)
- **Git** (pre-installed on Ubuntu 24.04)
- **Docker and Docker Compose** — if not installed, follow Step 1 below

You do **not** need Go, Python, Java, or any other language runtime.

---

## Step 1: Install Docker

If Docker is already installed, skip to Step 2.

Run the following commands one at a time in your terminal:

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

**Important**: Log out and log back in (or run `newgrp docker`) for the group change to take effect.

Verify Docker is working:

```bash
docker --version
docker run --rm hello-world
```

You should see "Hello from Docker!" in the output.

---

## Step 2: Download the CLI and System Components

All release assets are available on the GitHub release page:

> https://github.com/xueyulinn/cicd-system/releases/tag/v0.1.0-beta

### Option A: Download from the browser

If you have a desktop environment, open the link above in a browser, and download:

- **`cicd-linux-amd64`** (the CLI binary)
- **`compose-bundle-v0.1.0-beta.tar.gz`** (the backend services)

### Option B: Download from the command line

If you do not have a browser (e.g., Ubuntu Server), install the GitHub CLI and log in first:

```bash
sudo apt install -y gh
gh auth login
```

Follow the prompts: select **GitHub.com** → **HTTPS** → **Login with a web browser**. It will display a short code — open https://github.com/login/device on any other device and enter the code.

> If `sudo apt install -y gh` fails, see [Appendix: Install GitHub CLI](#appendix-install-github-cli) at the bottom of this document.

Then download all release assets:

```bash
gh release download v0.1.0-beta --repo xueyulinn/cicd-system
```

### Install the CLI

```bash
chmod +x cicd-linux-amd64
sudo mv cicd-linux-amd64 /usr/local/bin/cicd
```

Verify:

```bash
cicd --help
```

You should see a list of available commands.

---

## Step 3: Start the System Components

Extract the compose bundle and start the services:

```bash
tar -xzf compose-bundle-v0.1.0-beta.tar.gz
cd compose-bundle
cp .env.example .env
docker compose up -d
```

This will download and start all backend services. The first run may take a few minutes.

If your machine already uses PostgreSQL on host port `5432`, update the compose bundle before starting:

```bash
# edit docker-compose.yaml and change:
#   "5432:5432"
# to:
#   "55432:5432"
```

Then point any manual `DATABASE_URL` overrides at `127.0.0.1:55432` instead of `127.0.0.1:5432`.

---

## Step 4: Verify the Installation

Check that all containers are running:

```bash
docker compose ps
```

All services should show `Up` status. Then check the API gateway health:

```bash
curl http://localhost:8000/health
```

You should see a successful response.

For more detailed verification steps, see [`../verification-steps.md`](../verification-steps.md).

---

## Using the CLI

The CLI must be run inside a Git repository that contains a pipeline file.

**Validate** a pipeline file:

```bash
cicd verify .pipelines/pipeline.yaml
```

**Dry-run** (validate and simulate without executing):

```bash
cicd dryrun .pipelines/pipeline.yaml
```

**Run** a pipeline:

```bash
cicd run --file .pipelines/pipeline.yaml
```

For example pipelines that demonstrate success, failure, and validation errors, see the [README](../../README.md).

---

## Stop the System

```bash
cd ~/compose-bundle
docker compose down
```

To remove all data (database volumes, etc.):

```bash
docker compose down -v
```

---

## Troubleshooting

| Problem                                        | Solution                                                                 |
| ---------------------------------------------- | ------------------------------------------------------------------------ |
| `permission denied` when running `docker`      | Log out and log back in, or run `newgrp docker`                          |
| `connection refused` when running CLI commands | Make sure backend is running: `cd ~/compose-bundle && docker compose ps` |
| Reporting service cannot connect to Postgres | If port `5432` is already in use on the host, change the compose Postgres mapping to another host port such as `55432` before starting the system |
| Containers show `Exited` status                | Check logs: `docker compose logs <service-name>`                         |
| `cicd: command not found`                      | Run: `sudo mv cicd-linux-amd64 /usr/local/bin/cicd`                      |

---

## Appendix: Install GitHub CLI

If `sudo apt install -y gh` fails, use the official installation method:

```bash
(type -p wget >/dev/null || (sudo apt update && sudo apt-get install wget -y)) \
&& sudo mkdir -p -m 755 /etc/apt/keyrings \
&& out=$(mktemp) \
&& wget -nv -O$out https://cli.github.com/packages/githubcli-archive-keyring.gpg \
&& cat $out | sudo tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null \
&& sudo chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
&& echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
| sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
&& sudo apt update \
&& sudo apt install gh -y
```

Then run `gh auth login` and follow the prompts.
