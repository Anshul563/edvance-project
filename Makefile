.PHONY: help test build lint check infra-up app-up purchase-up infra-down infra-logs infra-ps infra-config migrate

COMPOSE_FILE := infrastructure/docker/docker-compose.yml
ENV_FILE := .env

help:
	@printf "Edvance local development commands\n\n"
	@printf "  make infra-up      Start PostgreSQL + Redis only.\n"
	@printf "  make app-up        Start the supported default application services.\n"
	@printf "  make purchase-up   Start the course-to-learning purchase services (requires Razorpay test keys).\n"
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
	@go test ./services/admin-api/... ./services/api-gateway/... ./services/analytics-service/... ./services/auth-service/... ./services/course-service/... ./services/learning-service/... ./services/commerce-service/... ./services/payment-service/...
	@if [ -x services/moderation-service/.venv/bin/python ]; then cd services/moderation-service && ./.venv/bin/python -m pytest -q tests; else echo "moderation-service venv missing; skipping"; fi
	@if [ -x services/recommendation-service/.venv/bin/python ]; then cd services/recommendation-service && ./.venv/bin/python -m pytest -q tests; else echo "recommendation-service venv missing; skipping"; fi

build:
	@cd services/admin-api && go build ./...
	@cd services/api-gateway && go build ./...
	@cd services/analytics-service && go build ./...
	@cd services/auth-service && go build ./...
	@cd services/course-service && go build ./...
	@cd services/learning-service && go build ./...
	@cd services/commerce-service && go build ./...
	@cd services/payment-service && go build ./...
	@if [ -f $(ENV_FILE) ]; then docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) build api-gateway admin-api auth-service analytics-service recommendation-service moderation-service course-service learning-service commerce-service payment-service; else echo "$(ENV_FILE) missing; create it from .env.example first"; fi

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
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d --wait postgres redis

app-up: infra-up
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d api-gateway admin-api auth-service analytics-service recommendation-service moderation-service

purchase-up:
	@if [ ! -f $(ENV_FILE) ]; then echo "Copy .env.example to .env before starting the stack."; exit 1; fi
	@docker compose --env-file $(ENV_FILE) --profile purchase --profile payment -f $(COMPOSE_FILE) up -d postgres redis api-gateway auth-service course-service learning-service commerce-service payment-service

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
	@set -a; . ./.env; set +a; docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d --wait postgres
	@set -a; . ./.env; set +a; docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) exec -T postgres psql -U "$${POSTGRES_USER:-postgres}" -d "$${POSTGRES_DB:-postgres}" -v ON_ERROR_STOP=1 < infrastructure/postgres/init/01-create-databases.sql
	@set -e; set -a; . ./.env; set +a; for entry in "auth-service edvance_auth" "analytics-service edvance_analytics" "course-service edvance_course" "learning-service edvance_learning" "commerce-service edvance_commerce" "payment-service edvance_payment"; do \
		set -- $$entry; service=$$1; database=$$2; \
		for migration in services/$$service/migrations/*.sql; do \
			docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) exec -T postgres psql -U "$${POSTGRES_USER:-postgres}" -d "$$database" -v ON_ERROR_STOP=1 < "$$migration"; \
		done; \
	done
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) run --build --rm --no-deps recommendation-service alembic upgrade head
	@docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) run --build --rm --no-deps moderation-service alembic upgrade head
