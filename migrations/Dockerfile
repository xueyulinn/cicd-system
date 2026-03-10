FROM postgres:16-alpine

WORKDIR /migrations

COPY migrations/001_report_store_schema.sql /migrations/001_report_store_schema.sql

# Defaults target the compose postgres service; override with env when needed.
ENV DB_HOST=postgres
ENV DB_PORT=5432
ENV DB_USER=cicd
ENV DB_NAME=reportstore

CMD ["sh", "-ec", "psql -h \"$DB_HOST\" -p \"$DB_PORT\" -U \"$DB_USER\" -d \"$DB_NAME\" -f /migrations/001_report_store_schema.sql"]
