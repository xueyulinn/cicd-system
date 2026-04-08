# e-team Helm Chart

This chart packages the e-team stateless services for Kubernetes and can optionally deploy the report database inside the cluster.

## Components

Kubernetes-enabled by this chart:

- `api-gateway`
- `validation-service`
- `execution-service`
- `worker-service`
- `reporting-service`
- `rabbitmq` Deployment, Service, and credentials `Secret` when `rabbitmq.enabled=true` (AMQP for pipeline job dispatch; aligns with Docker Compose)
- `postgres` StatefulSet, Service, Secret, and migration `Job` when `postgres.enabled=true`

Execution and worker receive `RABBITMQ_URL` from the RabbitMQ `Secret`. The worker also gets `EXECUTION_URL` (HTTP callbacks to execution) and `WORKER_CONCURRENCY` (RabbitMQ consumers per Pod, for parallel-ready jobs). Override `workerService.concurrency` in `values.yaml` as needed.

**Docker Compose:** `scripts/gen-compose-env-from-values.rb` reads the same `values.yaml` and writes `compose.values.env` (including `RABBITMQ_*`, `EXECUTION_URL`, `WORKER_CONCURRENCY`) so local Compose stays aligned with these defaults.

Not Kubernetes-enabled in this chart:

- None of the current application components are excluded, but the worker still depends on a reachable Docker socket. In Minikube that is provided through a `hostPath` mount to `/var/run/docker.sock`.

## Build Images

Build the current repo images into Minikube before installing the chart:

```bash
minikube image build -t e-team-api-gateway:latest -f cmd/api-gateway/Dockerfile .
minikube image build -t e-team-validation-service:latest -f cmd/validation-service/Dockerfile .
minikube image build -t e-team-execution-service:latest -f cmd/execution-service/Dockerfile .
minikube image build -t e-team-worker-service:latest -f cmd/worker-service/Dockerfile .
minikube image build -t e-team-reporting-service:latest -f cmd/reporting-service/Dockerfile .
minikube image build -t e-team-db-migrate:latest -f migrations/Dockerfile .
```

If you prefer a remote registry, override the image repositories/tags in `values.yaml` or with `--set`.

### CI-built images (GitHub Container Registry)

On push to `main` and on semantic version tags, [`.github/workflows/publish-images.yaml`](../../.github/workflows/publish-images.yaml) builds and pushes all application images to GHCR using **Docker Buildx** with **`linux/amd64`** and **`linux/arm64`** (manifest list per tag, suitable for Intel/AMD and Apple Silicon clusters).

`ghcr.io/<lowercase GitHub org>/<lowercase repo>/<component>`

Tags include the **short git SHA**, **`sha-<full SHA>`**, **`main`** when built from `main`, and the **git tag** on release tag pushes.

Default `values.yaml` points at `ghcr.io/cs7580-sea-sp26/e-team/*` with tag **`main`**. If your fork uses a different org/repo, update the `repository` fields (the workflow always publishes under your actual `GITHUB_REPOSITORY`).

Pin Helm to an exact image: `--set executionService.image.tag=<short-sha>` (and the same pattern for other services / migration).

#### Private GHCR images (pull credentials)

If packages are **private** (common for org-managed GHCR), nodes need credentials to pull images. Use a GitHub **Personal Access Token** (classic or fine-grained) with at least **`read:packages`**.

```bash
kubectl -n e-team create secret docker-registry ghcr-pull-secret \
  --docker-server=ghcr.io \
  --docker-username=<YOUR_GITHUB_USERNAME> \
  --docker-password=<YOUR_TOKEN>
```

Then install or upgrade Helm (quote `--set` in **zsh**):

```bash
helm upgrade --install e-team ./charts/e-team -n e-team \
  --set 'global.imagePullSecrets[0].name=ghcr-pull-secret'
```

## Install

Install into a namespace:

```bash
helm install e-team ./charts/e-team -n e-team --create-namespace
```

If the release already exists and you rebuilt local Minikube images, apply the latest chart/templates with:

```bash
helm upgrade e-team ./charts/e-team -n e-team
```

If you want to use an external Postgres instance instead of the in-cluster StatefulSet:

```bash
helm install e-team ./charts/e-team \
  -n e-team --create-namespace \
  --set postgres.enabled=false \
  --set externalDatabase.url='postgres://cicd:cicd@host:5432/reportstore?sslmode=disable' \
  --set externalDatabase.host='host' \
  --set externalDatabase.port=5432 \
  --set externalDatabase.username='cicd' \
  --set externalDatabase.password='cicd' \
  --set externalDatabase.database='reportstore'
```

## Upgrade

```bash
helm upgrade e-team ./charts/e-team -n e-team
```

The migration job runs as a `post-install,post-upgrade` hook when `postgres.enabled=true`, so the Postgres Secret/Service/StatefulSet are created first and the hook waits for the database to become ready before applying SQL.

## Uninstall

```bash
helm uninstall e-team -n e-team
```

The chart is intended to preserve the PostgreSQL PVC so report data is not removed accidentally. Remove the PVC manually only if you want to destroy the stored report data.

## Logs

```bash
kubectl -n e-team logs deploy/e-team-e-team-api-gateway
kubectl -n e-team logs deploy/e-team-e-team-validation-service
kubectl -n e-team logs deploy/e-team-e-team-execution-service
kubectl -n e-team logs deploy/e-team-e-team-reporting-service
kubectl -n e-team logs deploy/e-team-e-team-worker-service
kubectl -n e-team logs deploy/e-team-e-team-rabbitmq
kubectl -n e-team logs job/e-team-e-team-report-db-migrate
```

## Local CLI Against Kubernetes

The CLI still runs on your local machine. To make it talk to the Kubernetes deployment, port-forward the API gateway and point the CLI at the forwarded address:

```bash
kubectl -n e-team port-forward --address 127.0.0.1 svc/e-team-e-team-api-gateway 18000:8000
```

In another terminal:

```bash
export GATEWAY_URL=http://127.0.0.1:18000
./bin/cicd.exe verify .pipelines/pipeline.yaml
./bin/cicd.exe report --pipeline "Default Pipeline"
```

This setup was validated for `verify` and `report` against the Helm/Kubernetes deployment.

## Observability

Enable the observability stack in Kubernetes with:

```bash
helm upgrade --install e-team ./charts/e-team \
  -n e-team --create-namespace \
  --set observability.enabled=true
```

This deploys:

- Prometheus
- Loki
- Tempo
- OpenTelemetry Collector
- Promtail
- Grafana

Observability data is persisted via PVCs for Prometheus, Loki, Tempo, and Grafana.

### Access Grafana

```bash
kubectl -n e-team port-forward svc/e-team-e-team-grafana 3000:3000
```

Then open `http://localhost:3000` and log in with:

- username: `admin`
- password: `admin`

### Provisioned Datasources and Dashboards

Grafana is provisioned automatically with datasources for:

- Prometheus
- Loki
- Tempo

It also provisions these dashboards from config committed to the repository:

- `Pipeline Overview`
- `Stage and Job Breakdown`
- `Logs Viewer`
- `Trace Explorer`
- `HTTP Latency (Server & Client)`
- `Parallel execution & RabbitMQ`

This deployment uses Prometheus direct scraping for `/metrics`, Promtail for pod log shipping to Loki, and the OTel Collector for traces. That is the documented substitution for the recommended “single ingestion point” design, chosen so service logs and worker-managed job-container logs are both queryable without modifying pipeline job images.

### Observability Validation

```bash
kubectl -n e-team get pods | grep -E 'grafana|prometheus|loki|tempo|otel|promtail'
kubectl -n e-team get pvc
kubectl -n e-team port-forward svc/e-team-e-team-otel-collector 13133:13133
curl http://localhost:13133/
curl -u admin:admin http://localhost:3000/api/datasources
curl -u admin:admin http://localhost:3000/api/search
```

In Kubernetes, Promtail scrapes pod logs from the node and forwards them to Loki so both service logs and job-container logs are queryable in Grafana.

## Minikube Validation

1. Start Minikube and enable ingress if you want host-based access:

```bash
minikube start
minikube addons enable ingress
```

2. Install the chart:

```bash
minikube image build -t e-team-api-gateway:latest -f cmd/api-gateway/Dockerfile .
minikube image build -t e-team-validation-service:latest -f cmd/validation-service/Dockerfile .
minikube image build -t e-team-execution-service:latest -f cmd/execution-service/Dockerfile .
minikube image build -t e-team-worker-service:latest -f cmd/worker-service/Dockerfile .
minikube image build -t e-team-reporting-service:latest -f cmd/reporting-service/Dockerfile .
minikube image build -t e-team-db-migrate:latest -f migrations/Dockerfile .
helm install e-team ./charts/e-team -n e-team --create-namespace
```

3. Wait for the stack:

```bash
kubectl -n e-team get pods
kubectl -n e-team wait --for=condition=available deploy/e-team-e-team-api-gateway --timeout=180s
kubectl -n e-team wait --for=condition=available deploy/e-team-e-team-validation-service --timeout=180s
kubectl -n e-team wait --for=condition=available deploy/e-team-e-team-execution-service --timeout=180s
kubectl -n e-team wait --for=condition=available deploy/e-team-e-team-reporting-service --timeout=180s
kubectl -n e-team wait --for=condition=available deploy/e-team-e-team-worker-service --timeout=180s
kubectl -n e-team wait --for=condition=complete job/e-team-e-team-report-db-migrate --timeout=300s
```

4. Verify the report DB:

```bash
K8S_NAMESPACE=e-team ./scripts/verify-report-db-k8s.sh
```

5. Port-forward the gateway if not using ingress:

```bash
kubectl -n e-team port-forward --address 127.0.0.1 svc/e-team-e-team-api-gateway 18000:8000
```

6. Point the local CLI at the forwarded gateway:

```bash
export GATEWAY_URL=http://127.0.0.1:18000
```

7. Run a validation and reporting check:

```bash
./bin/cicd.exe verify .pipelines/pipeline.yaml
./bin/cicd.exe report --pipeline "Default Pipeline"
```

8. Optional execution check:

```bash
./bin/cicd.exe run --file .pipelines/pipeline.yaml
```

At the time of writing, `run` reaches the Kubernetes execution and worker services correctly, but successful job execution requires the worker to clone the repository revision. For private repositories, that currently needs Git credentials to be provided to the worker pod.

## Common Failure Diagnosis

- Missing env vars: inspect the generated ConfigMaps with `kubectl -n e-team get configmap -o yaml`
- Migration job failed: `kubectl -n e-team logs job/e-team-e-team-report-db-migrate`
- Image architecture mismatch: rebuild/load images for the Minikube node architecture
- Readiness probe returns `404`: you are likely using stale service images; rebuild the local Minikube images from this repository and run `helm upgrade`
- Worker startup failures: confirm `/var/run/docker.sock` exists inside the node and the `hostPath` mount is allowed
- `run` fails with Git clone/authentication errors: the worker is trying to clone the repository revision inside Kubernetes; public repos work more easily, while private repos need credentials injected into the worker
- External DB mode misconfigured: when `postgres.enabled=false`, set `externalDatabase.url`; if the DB wait init containers remain enabled, also set `externalDatabase.host`, `port`, `username`, `password`, and `database`
- Observability pods fail to start: inspect `kubectl -n e-team logs deploy/e-team-e-team-grafana`, `kubectl -n e-team logs deploy/e-team-e-team-otel-collector`, and `kubectl -n e-team logs ds/e-team-e-team-promtail`
