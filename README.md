# Simple E-Commerce System

Golang, Gin, PostgreSQL based e-commerce MVP.

## Member APIs

- `POST /auth/register`
- `POST /auth/login`
- `GET /users/me`
- `GET /health`

## Run locally

```bash
docker compose up --build
```

The API listens on `http://localhost:8080`.

Swagger UI is available at `http://localhost:8080/swagger/index.html`.

Regenerate the Swagger specification after changing API annotations:

```bash
make swagger
```
