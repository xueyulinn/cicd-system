#!/bin/sh

set -eu

if [ -z "${DB_PASSWORD:-}" ]; then
  echo "DB_PASSWORD is required"
  exit 1
fi

export MYSQL_PWD="${DB_PASSWORD}"

echo "Waiting for MySQL at ${DB_HOST}:${DB_PORT} ..."
until mysqladmin --protocol=TCP -h "${DB_HOST}" -P "${DB_PORT}" -u "${DB_USER}" ping --silent >/dev/null 2>&1; do
  sleep 2
done

mysql --protocol=TCP -h "${DB_HOST}" -P "${DB_PORT}" -u "${DB_USER}" "${DB_NAME}" -e \
  "CREATE TABLE IF NOT EXISTS schema_migrations (
      filename VARCHAR(255) NOT NULL PRIMARY KEY,
      applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
   );"

for f in $(find /migrations -maxdepth 1 -name '*.sql' | sort); do
  name="$(basename "$f")"
  applied="$(mysql --protocol=TCP -h "${DB_HOST}" -P "${DB_PORT}" -u "${DB_USER}" "${DB_NAME}" -N -B -e "SELECT COUNT(*) FROM schema_migrations WHERE filename = '${name}'")"
  if [ "${applied}" = "0" ]; then
    echo "Applying ${name}"
    mysql --protocol=TCP -h "${DB_HOST}" -P "${DB_PORT}" -u "${DB_USER}" "${DB_NAME}" < "${f}"
    mysql --protocol=TCP -h "${DB_HOST}" -P "${DB_PORT}" -u "${DB_USER}" "${DB_NAME}" -e \
      "INSERT INTO schema_migrations (filename) VALUES ('${name}')"
  else
    echo "Skipping ${name} (already applied)"
  fi
done

echo "Migrations completed."
