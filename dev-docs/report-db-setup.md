# Report store database setup

PostgreSQL is used to store pipeline run, stage, and job data for the `report` subcommand. Execution and report services connect via a connection string in the environment.

## Connection config

Set one of these environment variables (execution and report services read it):

- **`DATABASE_URL`** – full connection URL, e.g.  
  `postgres://cicd:cicd@localhost:5432/reportstore?sslmode=disable`
- **`REPORT_DB_URL`** – alternative name; same format as above.

Example for local dev (after starting Postgres with the compose file below):

```bash
export DATABASE_URL="postgres://cicd:cicd@localhost:5432/reportstore?sslmode=disable"
```

## Run PostgreSQL (development)

From the repo root, align env with the Helm chart defaults (see `charts/e-team/values.yaml`):

```bash
ruby scripts/gen-compose-env-from-values.rb   # writes compose.values.env
docker compose --env-file compose.values.env up -d
```

Default credentials and image tags match `compose.values.env` (typically user `cicd`, database `reportstore`). Host from your machine: `localhost`, port `5432`.

Stop:

```bash
docker compose --env-file compose.values.env down
```

## Kubernetes (Helm chart or rendered manifests)

Postgres, the migration Job, and app Deployments are defined in [`charts/e-team/`](../charts/e-team/). For `kubectl apply` without Helm, use the checked-in render:

```bash
kubectl create namespace e-team --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f k8s/helm-rendered/e-team.yaml -n e-team
```

Regenerate that file after chart changes: `./scripts/render-k8s-manifests.sh`.

See [`k8s/README.md`](../k8s/README.md) and [`charts/e-team/README.md`](../charts/e-team/README.md) for Helm installs and troubleshooting.

After Postgres and the migrate Job are up, you can verify tables/columns in-cluster with:

```bash
./scripts/verify-report-db-k8s.sh
```

Execution and reporting services resolve Postgres via the in-cluster Service from the chart (e.g. `e-team-postgres` when using `fullnameOverride=e-team` in the render script).

## Apply schema

After Postgres is running, create the report store tables and indexes once:

```bash
psql "$DATABASE_URL" -f migrations/001_report_store_schema.sql
```

If you don’t have `psql` installed, run the SQL inside the container (from repo root so the file path is available):

```bash
docker compose --env-file compose.values.env exec -T postgres psql -U cicd -d reportstore -f - < migrations/001_report_store_schema.sql
```

If the repo is not mounted, copy the migration into the container and run it, or paste the DDL from `migrations/001_report_store_schema.sql` into `psql` manually.

**Acceptance:** The three tables `pipeline_runs`, `stage_runs`, and `job_runs` exist with the expected columns and constraints (see `dev-docs/design/design-db-schema.md`).

## Verifying the setup

**Option 1 – Script (from repo root):**

```bash
./scripts/verify-report-db.sh
```

The script starts Postgres if needed, applies the migration, and checks that the three tables exist and are queryable. Exit code 0 means success.

**Option 2 – Manual checks:**

1. Start Postgres and apply the schema (see above).
2. Connect and list tables:
   ```bash
   docker compose --env-file compose.values.env exec postgres psql -U cicd -d reportstore -c "\dt"
   ```
   You should see `pipeline_runs`, `stage_runs`, `job_runs`.
3. Check columns for one table:
   ```bash
   docker compose --env-file compose.values.env exec postgres psql -U cicd -d reportstore -c "\d pipeline_runs"
   ```
   Confirm columns: `id`, `pipeline`, `run_no`, `start_time`, `end_time`, `status`, `git_hash`, `git_branch`, `git_repo`, and the unique constraint on `(pipeline, run_no)`.
4. (Optional) From the host with `psql` and `DATABASE_URL` set:
   ```bash
   psql "$DATABASE_URL" -c "SELECT 1 FROM pipeline_runs LIMIT 1;"
   ```
   No error means the table is readable.

## CI (optional)

For CI, start Postgres before tests that need the DB, e.g.:

```yaml
# example GitHub Actions
services:
  postgres:
    image: postgres:16-alpine
    env:
      POSTGRES_USER: cicd
      POSTGRES_PASSWORD: cicd
      POSTGRES_DB: reportstore
    ports:
      - 5432:5432
    options: >-
      --health-cmd "pg_isready -U cicd -d reportstore"
      --health-interval 10s
      --health-timeout 5s
      --health-retries 5
```

Then set `DATABASE_URL=postgres://cicd:cicd@localhost:5432/reportstore?sslmode=disable` in the CI env and run the migration before service tests.
