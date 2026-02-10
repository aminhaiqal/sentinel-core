# Variables
COMPOSE_FILE := scripts/docker-compose.yml
COMPOSE_BIN := podman compose 

APP_DIR := .
APP_BIN := bin/app

.PHONY: help infra-up infra-down infra-restart infra-logs clean

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

infra-up: ## Start all infrastructure containers (Postgres, Redis, Qdrant, Clickhouse)
	$(COMPOSE_BIN) -f $(COMPOSE_FILE) up -d

infra-down: ## Stop and remove all infrastructure containers
	$(COMPOSE_BIN) -f $(COMPOSE_FILE) down

infra-restart: infra-down infra-up ## Restart the infrastructure

infra-logs: ## Tail logs for all services
	$(COMPOSE_BIN) -f $(COMPOSE_FILE) logs -f

infra-clean: ## Remove all containers and volumes (Warning: Deletes your DB data)
	$(COMPOSE_BIN) -f $(COMPOSE_FILE) down -v

run: ## Run Go app locally (go run)
	go run $(APP_DIR)/cmd/server/main.go

build: ## Build Go binary
	mkdir -p bin
	go build -o $(APP_BIN) $(APP_DIR)/cmd/server/main.go

start: infra-up app-run

app-test: ## Run tests
	go test ./...

app-clean: ## Remove built binaries
	rm -rf bin