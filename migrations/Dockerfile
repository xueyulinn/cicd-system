FROM mysql:8.0

WORKDIR /migrations

# All *.sql files are applied in sorted order (001, 002, ...).
COPY migrations/*.sql /migrations/
COPY migrations/apply-migrations.sh /usr/local/bin/apply-migrations.sh
RUN chmod +x /usr/local/bin/apply-migrations.sh

# Defaults target the compose / in-cluster Service name `mysql`; override with env when needed.
ENV DB_HOST=mysql
ENV DB_PORT=3306
ENV DB_USER=cicd
ENV DB_NAME=reportstore

# DB_PASSWORD must be set at runtime (compose / K8s Secret).
CMD ["/usr/local/bin/apply-migrations.sh"]
