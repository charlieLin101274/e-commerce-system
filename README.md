# Simple E-Commerce System

Golang, Gin, PostgreSQL based e-commerce MVP.

The staged PRD for Campaign, Rule Engine, Notification, Cart Recall, and
Repurchase is available in [docs/prd](docs/prd/README.md).

## Member APIs

- `POST /auth/register`
- `POST /auth/login`
- `GET /users/me`
- `GET /health`

## Commerce APIs

- `GET /products`
- `GET /products/:id`
- `GET|POST /admin/products`
- `PUT|DELETE /admin/products/:id`
- `GET /cart`
- `POST /cart/items`
- `PUT|DELETE /cart/items/:id`
- `POST|GET /orders`
- `GET /orders/:id`

## Run locally

```bash
docker compose up --build
```

The API listens on `http://localhost:8080`.

Swagger UI is available at `http://localhost:8080/swagger/index.html`.

## Local admin

The database migration creates a local admin account:

- Email: `admin@example.com`
- Password: `Admin123!`

This credential is intended for local development only. Replace the seed
strategy before deploying the application to a shared or production
environment.

Regenerate the Swagger specification after changing API annotations:

```bash
make swagger
```
