.PHONY: help test build lint check infra-up infra-down infra-logs infra-ps infra-config migrate

COMPOSE_FILE := infrastructure/docker/docker-compose.yml
ENV_FILE := .env

help:
	@printf "Edvance local development commands\n\n"
	@printf "  make infra-up      Start PostgreSQL + Redis and the supported default services.\n"
	@printf "  make infra-down    Stop the local stack without removing persistent volumes.\n"
	@printf "  make infra-logs    Follow Docker Compose logs.\n"
	@printf "  make infra-ps      Show running containers.\n"
	@printf "  make infra-config  Validate the compose configuration.\n"
	@printf "  make migrate       Apply the local database migrations for the included services.\n"
	@printf "  make test          Run the verified Go and Python service tests.\n"
	@printf "  make build         Build the Go modules and Docker assets in the default profile.\n"
	@printf "  make lint          Run the project lint tasks available in the current environment.\n"
	@printf "  make check         Alias for test + build.\n"

test:
	@go test ./services/admin-api/... ./services/api-gateway/... ./services/analytics-service/...
	@if [ -x services/moderation-service/.venv/bin/python ]; then cd services/moderation-service && ./.venv/bin/python -m pytest -q tests; else echo "moderation-service venv missing; skipping"; fi
	@if [ -x services/recommendation-service/.venv/bin/python ]; then cd services/recommendation-service && ./.venv/bin/python -m pytest -q tests; else echo "recommendation-service venv missing; skipping"; fi

build:
	@cd services/admin-api && go build ./...
	@cd services/api-gateway && go build ./...
	@cd services/analytics-service && go build ./...
	@if [ -f $(ENV_FILE) ]; then docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) build api-gateway admin-api auth-service analytics-service recommendation-service moderation-service; else echo "$(ENV_FILE) missing; create it from .env.example first"; fi

lint:
	@command -v gofmt >/dev/null && gofmt -l services/admin-api services/api-gateway services/analytics-service || true
	@if command -v ruff >/dev/null; then \
		cd services/moderation-service && ruff check app tests; \
		cd ../recommendation-service && ruff check app tests; \
	else \
		echo "ruff not installed; skipping Python lint"; \
	fi

check: test build

infra-up:
	@if [ ! -f $(ENV_FILE) ]; then echo "Copy .env.example to .env before starting the stack."; exit 1; fi
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d postgres redis api-gateway admin-api auth-service analytics-service recommendation-service moderation-service

infra-down:
	@if [ ! -f $(ENV_FILE) ]; then echo "Copy .env.example to .env before stopping the stack."; exit 1; fi
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) down

infra-logs:
	@if [ ! -f $(ENV_FILE) ]; then echo "Copy .env.example to .env before viewing logs."; exit 1; fi
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) logs -f

infra-ps:
	@if [ ! -f $(ENV_FILE) ]; then echo "Copy .env.example to .env before checking service status."; exit 1; fi
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) ps

infra-config:
	@if [ ! -f $(ENV_FILE) ]; then echo "Copy .env.example to .env before validating compose configuration."; exit 1; fi
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) config

migrate:
	@if [ ! -f $(ENV_FILE) ]; then echo "Copy .env.example to .env before applying migrations."; exit 1; fi
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d postgres redis
	@for f in services/auth-service/migrations/*.sql; do \
		docker run --rm --network host -e POSTGRES_USER=$${POSTGRES_USER:-postgres} -e POSTGRES_PASSWORD=$${POSTGRES_PASSWORD:-postgres} -v "$$(pwd)/$$f:/tmp/migration.sql:ro" postgres:16-alpine sh -lc 'psql "postgresql://$${POSTGRES_USER:-postgres}:$${POSTGRES_PASSWORD:-postgres}@127.0.0.1:$${POSTGRES_PORT:-5432}/edvance_auth" -v ON_ERROR_STOP=1 -f /tmp/migration.sql'; \
	done
	@for f in services/analytics-service/migrations/*.sql; do \
		docker run --rm --network host -e POSTGRES_USER=$${POSTGRES_USER:-postgres} -e POSTGRES_PASSWORD=$${POSTGRES_PASSWORD:-postgres} -v "$$(pwd)/$$f:/tmp/migration.sql:ro" postgres:16-alpine sh -lc 'psql "postgresql://$${POSTGRES_USER:-postgres}:$${POSTGRES_PASSWORD:-postgres}@127.0.0.1:$${POSTGRES_PORT:-5432}/edvance_analytics" -v ON_ERROR_STOP=1 -f /tmp/migration.sql'; \
	done
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) run --rm --no-deps recommendation-service sh -lc 'DATABASE_URL=$${RECOMMENDATION_DATABASE_URL:-postgresql+asyncpg://postgres:postgres@postgres:5432/edvance_recommendation} alembic upgrade head'
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) run --rm --no-deps moderation-service sh -lc 'DATABASE_URL=$${MODERATION_DATABASE_URL:-postgresql+asyncpg://postgres:postgres@postgres:5432/edvance_moderation} alembic upgrade head'
