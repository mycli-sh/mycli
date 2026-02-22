.PHONY: build-api build-cli build-web build-site build-docs test lint dev dev-web dev-site dev-docs migrate sqlc-generate dev-reset docker-build-api docker-build-web

build-api:
	go build -o bin/api ./api/cmd/api

build-cli:
	go build -o bin/my ./cli/cmd/my

test:
	go test ./...

lint:
	golangci-lint run ./...

dev: build-api
	DATABASE_URL="postgres://mycli:mycli@localhost:5432/mycli_dev?sslmode=disable" \
	JWT_SECRET=dev-secret \
	BASE_URL=http://localhost:8080 \
	PORT=8080 \
	./bin/api

build-web:
	cd web && bun run build

dev-web:
	cd web && bun run dev

migrate:
	DATABASE_URL="postgres://mycli:mycli@localhost:5432/mycli_dev?sslmode=disable" \
	go run ./api/cmd/api --migrate

sqlc-generate:
	sqlc generate -f api/internal/database/sqlc/sqlc.yaml

build-site:
	cd site && bun run build

dev-site:
	cd site && bun run dev

build-docs:
	cd docs && bun run build

dev-docs:
	cd docs && bun run dev --port 4322

dev-reset:
	./scripts/dev-reset.sh

docker-build-api:
	docker build -f docker/api.Dockerfile -t mycli-api:local .

docker-build-web:
	docker build -f docker/web.Dockerfile -t mycli-web:local .
