# Postgres on Kubernetes (StatefulSet + PVC) + migration Job

Deploys PostgreSQL for the report store **inside the cluster only** (ClusterIP Service, no external exposure by default), then applies SQL migrations via a **Job** (sorted `*.sql`: 001, 002, …).

## Prerequisites

- A Kubernetes cluster (e.g. `minikube start`)
- Default `StorageClass` available (minikube provides `standard`)
- **Migration image** built and available to the cluster (see below)

## Build the migration image

From the **repository root**:

```bash
docker build -f migrations/Dockerfile -t e-team-db-migrate:latest .
```

- **Minikube** (image not shared with host Docker):  
  `minikube image build -t e-team-db-migrate:latest -f migrations/Dockerfile .`  
  or `docker build ...` then `minikube image load e-team-db-migrate:latest`
- **Other registry**: tag/push and set the image in `migrate-job.yaml` (or a Kustomize `images` patch).

Adding a new migration: drop `migrations/00N_*.sql`, rebuild the image, delete the Job (see below), re-apply.

## Apply

```bash
kubectl apply -k k8s/postgres/
```

The migration Job’s **init container** waits until Postgres accepts connections before running `psql` for each SQL file (lexicographic order under `/migrations`).

## Verify Postgres

```bash
kubectl -n e-team get pods,svc,pvc
kubectl -n e-team wait pod -l app.kubernetes.io/name=postgres --for=condition=Ready --timeout=120s
```

```bash
kubectl -n e-team run pgcheck --rm -it --restart=Never --image=postgres:16-alpine -- \
  sh -c 'PGPASSWORD=cicd psql -h postgres -U cicd -d reportstore -c "SELECT 1"'
```

## Verify migrations

```bash
kubectl -n e-team wait job/report-db-migrate --for=condition=complete --timeout=300s
kubectl -n e-team logs job/report-db-migrate
```

Expect log lines `Applying /migrations/001_...` and `Applying /migrations/002_...` and exit 0.

## Re-run migrations

`Job` specs are mostly immutable. After changing SQL or the migrate image:

```bash
kubectl -n e-team delete job report-db-migrate --ignore-not-found
kubectl apply -k k8s/postgres/
```

## Connection string (in-cluster apps)

```
postgres://cicd:cicd@postgres.e-team.svc.cluster.local:5432/reportstore?sslmode=disable
```

Set `DATABASE_URL` or `REPORT_DB_URL` on execution/reporting workloads **after** the migrate Job has completed (or use an init container / ordering in a later change).

## Credentials

Default user/password/db match `docker-compose.yaml` (`cicd` / `cicd` / `reportstore`). **Do not use in production** — replace `secret.yaml` or use External Secrets.

## Docker Compose

`docker-compose.yaml` `db-migrate` uses the same `migrations/Dockerfile`; it now applies **all** `migrations/*.sql` in order when the container runs.

## Next (optional)

- Wire execution/reporting Deployments to wait for migrations + use internal DNS
- CI: build/push `e-team-db-migrate` (or your registry) on migration changes
