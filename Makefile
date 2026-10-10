.PHONY: help test build lint check infra-up infra-down

help:
	@printf "Edvance Makefile\n\n"
	@printf "  make help        Show this help.\n"
	@printf "  make test        Run the relevant repo checks.\n"
	@printf "  make build       Build the Go modules that exist in the workspace.\n"
	@printf "  make lint        Run available lint checks for configured tools.\n"
	@printf "  make check       Alias for test + build.\n"
	@printf "  make infra-up    Informational: no compose stack is present in infrastructure/docker.\n"
	@printf "  make infra-down  Informational: local infrastructure must be stopped separately.\n"

test:
	@go test ./services/admin-api/... ./services/api-gateway/...
	@if [ -x services/moderation-service/.venv/bin/python ]; then cd services/moderation-service && ./.venv/bin/python -m pytest -q tests; else echo "moderation-service venv missing; skipping"; fi
	@if [ -x services/recommendation-service/.venv/bin/python ]; then cd services/recommendation-service && ./.venv/bin/python -m pytest -q tests; else echo "recommendation-service venv missing; skipping"; fi

build:
	@cd services/admin-api && go build ./...
	@cd services/api-gateway && go build ./...
	@cd services/analytics-service && go build ./...

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
	@echo "No docker compose stack is configured in infrastructure/docker; start Postgres/Redis/NATS manually."
	@exit 1

infra-down:
	@echo "No docker compose stack is configured in infrastructure/docker; stop local infrastructure manually."
	@exit 1
