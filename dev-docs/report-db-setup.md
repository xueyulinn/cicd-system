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

From the repo root:

```bash
docker compose up -d
```

Default (from `docker-compose.yaml`):

- Host: `localhost`, port: `5432`
- User: `cicd`, password: `cicd`, database: `reportstore`

Stop:

```bash
docker compose down
```

## Apply schema

After Postgres is running, create the report store tables and indexes once:

```bash
psql "$DATABASE_URL" -f migrations/001_report_store_schema.sql
```

If you don’t have `psql` installed, run the SQL inside the container (from repo root so the file path is available):

```bash
docker compose exec -T postgres psql -U cicd -d reportstore -f - < migrations/001_report_store_schema.sql
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
   docker compose exec postgres psql -U cicd -d reportstore -c "\dt"
   ```
   You should see `pipeline_runs`, `stage_runs`, `job_runs`.
3. Check columns for one table:
   ```bash
   docker compose exec postgres psql -U cicd -d reportstore -c "\d pipeline_runs"
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
