COMPOSE_FILE := docker-compose.yml
COMPOSE := docker compose -f $(COMPOSE_FILE)

.PHONY: help local-start local-stop local-restart local-logs local-ps local-rebuild local-clean

help:
	@echo "Available targets:"
	@echo "  make local-start   - Build and start postgres, backend, frontend"
	@echo "  make local-stop    - Stop and remove containers"
	@echo "  make local-restart - Restart all services"
	@echo "  make local-logs    - Tail logs for all services"
	@echo "  make local-ps      - Show running service status"
	@echo "  make local-rebuild - Force rebuild and restart"
	@echo "  make local-clean   - Stop services and remove volumes"

local-start:
	$(COMPOSE) up --build -d
	@echo "Local stack started: frontend=http://localhost:4200 backend=http://localhost:8080"

local-stop:
	$(COMPOSE) down

local-restart: local-stop local-start

local-logs:
	$(COMPOSE) logs -f --tail=200

local-ps:
	$(COMPOSE) ps

local-rebuild:
	$(COMPOSE) down
	$(COMPOSE) up --build --force-recreate -d

local-clean:
	$(COMPOSE) down -v
