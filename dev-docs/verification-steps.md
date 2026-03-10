## 6. Verifying That All System Components Are Installed Correctly

This document describes how an evaluator can confirm that the CLI, services, and report database are installed and wired together correctly.

### 6.1 Verify the CLI Binary

1. **Install from GitHub Release (recommended for evaluators)**

   ```bash
   curl -sSL https://raw.githubusercontent.com/CS7580-SEA-SP26/e-team/main/scripts/install.sh | sh
   ```

2. **Or install from a local clone**

   From the repository root:

   ```bash
   ./scripts/install.sh
   ```

3. **Check that the CLI is on `PATH` and working**

   ```bash
   cicd --help
   cicd verify --help
   cicd run --help
   cicd dryrun --help
   cicd report --help
   ```

All commands above should print help text and exit with status code `0`.  
This confirms that the **`cicd` CLI is correctly installed and available on the shell `PATH`**.

### 6.2 Verify Core Services Can Start

From the repository root:

```bash
./scripts/start-services.sh
```

Expected output includes URLs and PIDs for:

- Validation Service: `http://localhost:8001`
- API Gateway: `http://localhost:8000`
- Execution Service: `http://localhost:8002`
- Reporting Service: `http://localhost:8004`
- Worker Service: `http://localhost:8003`

Keep this terminal running.  
If any service fails to start, the script prints an error and exits with a non-zero status code.

### 6.3 Verify the Report Store Database

From the repository root:

```bash
./scripts/verify-report-db.sh
```

This script:

- Starts Postgres via `docker compose` (if not already running)
- Applies the schema from `migrations/001_report_store_schema.sql`
- Verifies that the tables `pipeline_runs`, `stage_runs`, and `job_runs` exist and are queryable

You should see:

```text
=== Report DB setup verified successfully ===
```

and the script should exit with status code `0`.  
This confirms that the **report database and migrations are correctly configured**.

### 6.4 End‑to‑End Sanity Check: Run a Successful Pipeline

With services running (via `./scripts/start-services.sh` in another terminal), run:

```bash
cicd run --file .pipelines/pipeline.yaml
```

This executes the default example pipeline **“Default Pipeline”**, which has stages `build`, `test`, and `deploy`.
You should see execution logs for each stage and job, and the command should end with:

```text
Run completed successfully.
```

and exit code `0`.  
This confirms that the **CLI, services, and execution flow are wired together correctly**.

