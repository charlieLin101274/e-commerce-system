.PHONY: up down test fmt swagger integration-up integration-down integration-test

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

integration-up:
	docker compose -p ecommerce-integration \
		-f docker-compose.yaml \
		-f integration-tests/compose.override.yaml \
		up -d --build

integration-down:
	docker compose -p ecommerce-integration \
		-f docker-compose.yaml \
		-f integration-tests/compose.override.yaml \
		down -v --remove-orphans

integration-test: integration-up
	@status=0; \
	go test -tags=integration ./integration-tests/suites/... || status=$$?; \
	if [ $$status -ne 0 ]; then \
		docker compose -p ecommerce-integration \
			-f docker-compose.yaml \
			-f integration-tests/compose.override.yaml \
			logs --no-color; \
	fi; \
	$(MAKE) integration-down; \
	exit $$status
