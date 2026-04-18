# Report Store Database Setup

MySQL 8 is used to store pipeline run, stage, and job data for the `report` subcommand.

## Connection Config

Execution and reporting services read one of:

- `DATABASE_URL`
- `REPORT_DB_URL`

Expected DSN format:

```bash
user:password@tcp(host:3306)/dbname?parseTime=true&charset=utf8mb4&loc=UTC
```

Local example:

```bash
export DATABASE_URL="cicd:cicd@tcp(localhost:3306)/reportstore?parseTime=true&charset=utf8mb4&loc=UTC"
```

## Run MySQL Locally

From repo root:

```bash
ruby scripts/gen-compose-env-from-values.rb
docker compose --env-file compose.values.env up -d mysql db-migrate
```

- `mysql` starts the DB.
- `db-migrate` applies only unapplied SQL files from `migrations/` and records them in `schema_migrations`.

Stop:

```bash
docker compose --env-file compose.values.env down
```

## Kubernetes

The Helm chart (`charts/e-team`) can run MySQL + migration job in-cluster.

Verify in-cluster DB setup:

```bash
./scripts/verify-report-db-k8s.sh
```

## Manual Verification

Check tables:

```bash
docker compose --env-file compose.values.env exec -T mysql \
  mysql -ucicd -pcicd reportstore -e "SHOW TABLES;"
```

Check core schema:

```bash
docker compose --env-file compose.values.env exec -T mysql \
  mysql -ucicd -pcicd reportstore -e "DESCRIBE pipeline_runs;"
```

Check queryability:

```bash
docker compose --env-file compose.values.env exec -T mysql \
  mysql -ucicd -pcicd reportstore -e "SELECT 1 FROM pipeline_runs LIMIT 1;"
```

## CI Note

In CI, start MySQL 8, set `DATABASE_URL` to the same DSN format, then run migrations before service tests.
