.PHONY: up down test fmt swagger

up:
	docker compose up --build

down:
	docker compose down

test:
	go test ./...

fmt:
	gofmt -w .

swagger:
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init \
		-g cmd/api/main.go \
		-o docs/swagger \
		--parseDependency \
		--parseInternal
