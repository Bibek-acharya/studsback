.PHONY: help install run build clean docker-up docker-down docker-logs docker-restart db-connect test

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install Go dependencies
	go mod download
	go mod tidy

run: ## Run the application
	go run main.go

build: ## Build the application
	go build -o bin/server main.go

clean: ## Clean build artifacts
	rm -rf bin/

docker-up: ## Start PostgreSQL with Docker
	sudo docker compose up -d

docker-down: ## Stop PostgreSQL container
	sudo docker compose down

docker-logs: ## Show PostgreSQL logs
	sudo docker compose logs -f postgres

docker-restart: ## Restart PostgreSQL container
	sudo docker compose restart

db-connect: ## Connect to PostgreSQL database
	sudo docker exec -it studsphere_db psql -U studsphere_user -d studsphere

test: ## Run tests
	go test -v ./...

dev: docker-up run ## Start database and run application
