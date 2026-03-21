# Issue 1: Postgres on Kubernetes (StatefulSet + PVC)

Deploys PostgreSQL for the report store **inside the cluster only** (ClusterIP Service, no external exposure by default).

## Prerequisites

- A Kubernetes cluster (e.g. `minikube start`)
- Default `StorageClass` available (minikube provides `standard`)

## Apply

```bash
kubectl apply -k k8s/postgres/
```

Order is handled by `kustomization.yaml` (namespace → secret → service → statefulset).

## Verify

```bash
kubectl -n e-team get pods,svc,pvc
kubectl -n e-team wait pod -l app.kubernetes.io/name=postgres --for=condition=Ready --timeout=120s
```

Check readiness from another pod in the same namespace:

```bash
kubectl -n e-team run pgcheck --rm -it --restart=Never --image=postgres:16-alpine -- \
  sh -c 'PGPASSWORD=cicd psql -h postgres -U cicd -d reportstore -c "SELECT 1"'
```

## Connection string (in-cluster apps)

```
postgres://cicd:cicd@postgres.e-team.svc.cluster.local:5432/reportstore?sslmode=disable
```

Set `DATABASE_URL` or `REPORT_DB_URL` to this value on execution/reporting services (after migration Job lands in a follow-up issue).

## Credentials

Default user/password/db match `docker-compose.yaml` (`cicd` / `cicd` / `reportstore`). **Do not use in production** — replace `secret.yaml` or use External Secrets.

## Next (separate issue)

- Migration `Job` for `migrations/001_*.sql` and `002_*.sql`
- Wire execution/reporting Deployments to wait for migrations + use internal DNS
