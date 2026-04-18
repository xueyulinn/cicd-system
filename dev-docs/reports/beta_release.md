# Beta Report

## Features Implemented

- CLI binary release for Linux and macOS
- CLI installation script for released binaries
- Pipeline YAML validation
- Pipeline dry-run execution planning
- Local pipeline run command
- API gateway service
- Validation service
- Execution service
- Worker service
- Reporting service
- MySQL 8-backed report store
- Database schema migration SQL
- Dockerfiles for backend services
- Docker Compose configuration for backend services
- Health check endpoints for services
- Service-to-service HTTP configuration through environment variables

## Implementation Limitations

- `scripts/install.sh` currently installs only the CLI binary and does not install the Docker Compose bundle.
- `scripts/install.sh` supports Linux and macOS only; Windows environments such as Git Bash are not supported.
- The current `docker-compose.yaml` still builds images locally and depends on repository-local files, so it is not yet a fully portable release bundle.
- The `db-migrate` service builds from `migrations/Dockerfile` (SQL files baked into the image); changing migrations requires rebuilding that image.
- MySQL 8 data persistence is not configured with a named volume, so recreated containers may lose database data.
- `worker-service` depends on Docker socket access and runs as root in local containerized setups, which is convenient for development but not ideal for hardened environments.
- Running the full stack still assumes the user is working from the repository directory.
- The release/install flow for services is not yet unified into a single end-user command.


