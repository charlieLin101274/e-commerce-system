FROM migrate/migrate:v4.18.3

COPY infra/postgres/migrations /migrations

ENTRYPOINT ["migrate", "-path", "/migrations"]
