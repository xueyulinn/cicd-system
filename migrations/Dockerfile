FROM postgres:16-alpine

WORKDIR /migrations

# All *.sql files are applied in sorted order (001, 002, …). Add new files under migrations/ and rebuild the image.
COPY migrations/*.sql /migrations/

# Defaults target the compose / in-cluster Service name `postgres`; override with env when needed.
ENV DB_HOST=postgres
ENV DB_PORT=5432
ENV DB_USER=cicd
ENV DB_NAME=reportstore

# PGPASSWORD must be set at runtime (compose / K8s Secret).
CMD ["sh", "-ec", "set -e; for f in $(find /migrations -maxdepth 1 -name '*.sql' | sort); do echo \"Applying $f\"; psql -h \"$DB_HOST\" -p \"$DB_PORT\" -U \"$DB_USER\" -d \"$DB_NAME\" -v ON_ERROR_STOP=1 -f \"$f\"; done"]
