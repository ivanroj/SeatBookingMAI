# Convenience targets for SeatBookingMAI.
# Run `make help` for the available commands.

GO ?= go
BACKEND_DIR := backend
DOCKER_COMPOSE ?= docker compose

.PHONY: help test cover vet fmt fmt-check build run lint compose-up compose-down compose-logs compose-rebuild

help: ## Show this help.
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "%-18s %s\n", $$1, $$2}'

test: ## Run all backend tests.
	cd $(BACKEND_DIR) && $(GO) test -count=1 ./...

cover: ## Run tests with coverage summary.
	cd $(BACKEND_DIR) && $(GO) test -count=1 -cover ./...

vet: ## Run go vet.
	cd $(BACKEND_DIR) && $(GO) vet ./...

fmt: ## Run gofmt -w on the backend.
	cd $(BACKEND_DIR) && $(GO) fmt ./...

fmt-check: ## Fail if any backend file is not gofmt-clean.
	@cd $(BACKEND_DIR) && diff=$$(gofmt -l .); \
		if [ -n "$$diff" ]; then \
			echo "Files not gofmt-clean:"; echo "$$diff"; exit 1; \
		fi

build: ## Build the backend binary into backend/server.
	cd $(BACKEND_DIR) && $(GO) build -o server ./cmd/server

run: ## Run the backend locally (requires DATABASE_URL).
	cd $(BACKEND_DIR) && $(GO) run ./cmd/server

lint: vet fmt-check ## Lightweight lint: gofmt + go vet.

compose-up: ## Start the full stack (db + backend + frontend) in the foreground.
	$(DOCKER_COMPOSE) up --build

compose-up-d: ## Start the full stack in the background.
	$(DOCKER_COMPOSE) up --build -d

compose-down: ## Stop the stack and remove containers (volumes kept).
	$(DOCKER_COMPOSE) down

compose-logs: ## Tail logs from all services.
	$(DOCKER_COMPOSE) logs -f --tail=100

compose-rebuild: ## Rebuild images without cache.
	$(DOCKER_COMPOSE) build --no-cache
