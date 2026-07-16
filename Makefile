.PHONY: up down test fmt

up:
	docker compose up --build

down:
	docker compose down

test:
	go test ./...

fmt:
	gofmt -w .
