.PHONY: help test test-unit test-integration test-contract test-all fmt lint build clean install-hooks

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Format Go code with gofmt
	@echo "==> Formatting Go code..."
	@gofmt -s -w .
	@echo "✓ Formatting complete"

lint: ## Run golangci-lint
	@echo "==> Running golangci-lint..."
	@golangci-lint run
	@echo "✓ Linting complete"

test-unit: ## Run unit tests
	@echo "==> Running unit tests..."
	@go test -v -race -coverprofile=coverage.out ./tests/unit/...
	@echo "✓ Unit tests complete"

test-integration: ## Run integration tests
	@echo "==> Running integration tests..."
	@go test -v -race ./tests/integration/...
	@echo "✓ Integration tests complete"

test-contract: ## Run contract tests
	@echo "==> Running contract tests..."
	@go test -v -race ./tests/contract/...
	@echo "✓ Contract tests complete"

test-all: test-unit test-integration test-contract ## Run all tests
	@echo "==> All tests complete"

test: test-unit ## Run unit tests (alias)

build: ## Build the bot binary
	@echo "==> Building bot..."
	@go build -o bin/bot ./cmd/bot
	@echo "✓ Build complete: bin/bot"

clean: ## Clean build artifacts
	@echo "==> Cleaning..."
	@rm -rf bin/ coverage.out
	@echo "✓ Clean complete"

install-hooks: ## Install git pre-commit hooks
	@echo "==> Installing git hooks..."
	@chmod +x scripts/pre-commit.sh
	@cp scripts/pre-commit.sh .git/hooks/pre-commit
	@echo "✓ Pre-commit hook installed"
	@echo ""
	@echo "Optional: Install pre-commit framework for more advanced hooks:"
	@echo "  pip install pre-commit"
	@echo "  pre-commit install"

pre-commit: fmt lint test-unit ## Run pre-commit checks (format, lint, test)
	@echo "✓ All pre-commit checks passed"

# Docker testing targets
docker-test: ## Run all tests in Docker
	@echo "==> Running tests in Docker..."
	@./scripts/test-with-docker.sh all

docker-test-unit: ## Run unit tests in Docker
	@echo "==> Running unit tests in Docker..."
	@./scripts/test-with-docker.sh unit

docker-test-integration: ## Run integration tests in Docker
	@echo "==> Running integration tests in Docker..."
	@./scripts/test-with-docker.sh integration

docker-test-contract: ## Run contract tests in Docker
	@echo "==> Running contract tests in Docker..."
	@./scripts/test-with-docker.sh contract

docker-test-multiwindow: ## Run multi-window specific tests in Docker
	@echo "==> Running multi-window tests in Docker..."
	@./scripts/test-with-docker.sh multiwindow

docker-up: ## Start Docker Compose test environment
	@echo "==> Starting Docker Compose test environment..."
	@docker compose -f docker-compose.test.yml up -d
	@echo "✓ Test environment started"
	@echo "  - PostgreSQL: localhost:5433"
	@echo "  - pgAdmin: http://localhost:5050 (admin@throttlebot.local / admin123)"

docker-down: ## Stop Docker Compose test environment
	@echo "==> Stopping Docker Compose test environment..."
	@docker compose -f docker-compose.test.yml down
	@echo "✓ Test environment stopped"

docker-clean: ## Clean Docker test environment (including volumes)
	@echo "==> Cleaning Docker test environment..."
	@docker compose -f docker-compose.test.yml down -v
	@echo "✓ Test environment cleaned"

docker-logs: ## Show Docker Compose logs
	@docker compose -f docker-compose.test.yml logs -f

docker-db-shell: ## Connect to PostgreSQL shell in Docker
	@docker compose -f docker-compose.test.yml exec postgres psql -U throttle -d throttlebot_test

docker-migrate: ## Run database migrations in Docker
	@echo "==> Running migrations..."
	@docker compose -f docker-compose.test.yml up migrate
	@echo "✓ Migrations complete"

docker-verify-schema: ## Verify multi-window database schema in Docker
	@echo "==> Verifying multi-window schema..."
	@docker compose -f docker-compose.test.yml exec -T postgres psql -U throttle -d throttlebot_test -c "\dt"
	@docker compose -f docker-compose.test.yml exec -T postgres psql -U throttle -d throttlebot_test -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name;"
	@echo "✓ Schema verification complete"

docker-seed-test-data: ## Seed test data in Docker database
	@echo "==> Seeding test data..."
	@docker compose -f docker-compose.test.yml exec -T postgres psql -U throttle -d throttlebot_test << 'EOSQL' \
		INSERT INTO groups (chat_id, char_limit, window_type, created_at, updated_at) \
		VALUES (-1001234567890, 1000, 'hour', NOW(), NOW()) \
		ON CONFLICT (chat_id) DO NOTHING; \
		INSERT INTO window_slots (chat_id, slot_id, window_type, char_limit, enabled, created_at, updated_at) \
		VALUES \
			(-1001234567890, 'a', 'hour', 100, true, NOW(), NOW()), \
			(-1001234567890, 'b', '6hour', 300, true, NOW(), NOW()), \
			(-1001234567890, 'c', 'day', 1000, true, NOW(), NOW()) \
		ON CONFLICT (chat_id, slot_id) DO UPDATE \
		SET enabled = EXCLUDED.enabled; \
		SELECT 'Test data seeded' as status; \
	EOSQL
	@echo "✓ Test data seeded"

# Phase 4 verification
phase4-verify: docker-up docker-migrate docker-verify-schema docker-test-multiwindow ## Verify Phase 4 implementation
	@echo ""
	@echo "=== Phase 4 Multi-Window Rate Enforcement Verification ==="
	@echo "✓ Docker environment started"
	@echo "✓ Database schema verified"
	@echo "✓ Multi-window tests executed"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Check test results above"
	@echo "  2. Connect to pgAdmin at http://localhost:5050"
	@echo "  3. Run 'make docker-db-shell' to inspect database"
	@echo "  4. Run 'make docker-down' when finished"

