PROJECT_NAME ?= bookshelf
DOCKER_BACKEND ?= tanjd/$(PROJECT_NAME)-backend:latest
DOCKER_FRONTEND ?= tanjd/$(PROJECT_NAME)-frontend:latest

GOLANGCI_VERSION ?= v2.4.0

MAKEFLAGS += --no-print-directory

.PHONY: help setup clean install-tools \
        backend-run frontend-run dev \
        migrate-new \
        test lint build check-ci \
        docker-build docker-run docker-push

.DEFAULT_GOAL := help

help: ## Show this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(PROJECT_NAME) — Available Commands:\n\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Development

setup: ## Install all deps (backend + frontend)
	cd backend && go mod download
	cd frontend && npm install
	pre-commit install

clean: ## Remove build artifacts (.next, backend bin)
	rm -rf frontend/.next backend/bin

# install-tools: ## Install dev tools (golangci-lint)
# 	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
# 	  | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_VERSION)

backend-run: ## Run Go backend (port 8000)
	cd backend && go run ./cmd/server

frontend-run: ## Run Next.js frontend (port 3000)
	cd frontend && npm run dev

dev: ## Run backend and frontend concurrently
	@echo "Starting backend on :8000 and frontend on :3000 ..."
	@trap 'kill 0' INT; \
	  $(MAKE) backend-run & \
	  $(MAKE) frontend-run & \
	  wait

migrate-new: ## Create a new migration pair (usage: make migrate-new NAME=add_foo_to_bar)
	@test -n "$(NAME)" || (echo "ERROR: NAME is required. Example: make migrate-new NAME=add_foo_to_bar"; exit 1)
	@SEQ=$$(ls backend/internal/db/migrations/*.up.sql 2>/dev/null \
	        | sed 's/.*\/0*//' | sed 's/_.*//' | sort -n | tail -1); \
	 NEXT=$$(printf "%06d" $$(( $${SEQ:-0} + 1 ))); \
	 touch "backend/internal/db/migrations/$${NEXT}_$(NAME).up.sql"; \
	 touch "backend/internal/db/migrations/$${NEXT}_$(NAME).down.sql"; \
	 echo "Created: $${NEXT}_$(NAME).{up,down}.sql"

##@ Testing & Quality

test: ## Run Go tests
	cd backend && go test ./...

lint: ## Lint Go code with golangci-lint
	cd backend && golangci-lint run ./...

build: ## Build Go binary
	cd backend && go build -o bin/server ./cmd/server

check-ci: lint test build ## Run all checks (used by CI)

##@ Docker

docker-build: ## Build backend and frontend Docker images
	docker build -t bookshelf-backend -f backend/Dockerfile backend
	docker build -t bookshelf-frontend -f frontend/Dockerfile frontend

docker-run: ## Run via docker compose (requires docker-compose.yml)
	docker compose up

docker-push: docker-build ## Build, tag, and push both images to Docker Hub
	docker tag bookshelf-backend:latest $(DOCKER_BACKEND)
	docker tag bookshelf-frontend:latest $(DOCKER_FRONTEND)
	docker push $(DOCKER_BACKEND)
	docker push $(DOCKER_FRONTEND)
