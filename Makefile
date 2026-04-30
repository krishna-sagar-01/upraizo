# ==============================================================
# Upraizo Database — Makefile
# ==============================================================
# Usage: make <target>
# Run 'make help' to see all available commands.
# ==============================================================

.PHONY: help dev-up dev-down dev-reset dev-logs server-dev server-build \
        prod-up prod-down prod-logs \
        migrate-up migrate-down migrate-status migrate-version migrate-create \
        backup restore health psql psql-prod \
        rmq-up rmq-down rmq-logs \
        deploy-setup deploy-network deploy-db deploy-server deploy-all \
        deploy-down deploy-logs deploy-status deploy-restart

# Default
.DEFAULT_GOAL := help

# ── Variables ─────────────────────────────────────────────────
APP_ENV       ?= dev
N             ?= 1
NAME          ?=
TIMESTAMP     ?=
COMPOSE_DEV   = docker compose -f db/docker/dev/docker-compose.yaml
COMPOSE_PROD  = docker compose -f db/docker/prod/docker-compose.yaml
COMPOSE_SRV   = docker compose -f server/docker-compose.prod.yml --env-file server/.env.prod

# ══════════════════════════════════════════════════════════════
# HELP
# ══════════════════════════════════════════════════════════════

help: ## Show this help
	@echo ""
	@echo "  Upraizo Database Commands"
	@echo "  ════════════════════════"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""

# ══════════════════════════════════════════════════════════════
# PRODUCTION
# ══════════════════════════════════════════════════════════════

prod-up: ## Start prod database (PG + PgBouncer)
	@test -f db/docker/prod/.env || (echo "❌ db/docker/prod/.env not found! Run: cp db/docker/prod/.env.example db/docker/prod/.env" && exit 1)
	$(COMPOSE_PROD) up -d
	@echo "✅ Prod DB running (PgBouncer on :6432)"

prod-down: ## Stop prod database (safely)
	$(COMPOSE_PROD) down

prod-logs: ## Show prod database logs
	$(COMPOSE_PROD) logs -f

# ══════════════════════════════════════════════════════════════
# MIGRATIONS
# ══════════════════════════════════════════════════════════════

migrate-up: ## Run pending UP migrations (APP_ENV=dev|prod)
	APP_ENV=$(APP_ENV) ./db/scripts/migrate.sh up

migrate-up-n: ## Run N UP migrations (make migrate-up-n N=5)
	APP_ENV=$(APP_ENV) ./db/scripts/migrate.sh up $(N)

migrate-down: ## Rollback N migrations (make migrate-down N=1)
	APP_ENV=$(APP_ENV) ./db/scripts/migrate.sh down $(N)

migrate-status: ## Show migration status
	APP_ENV=$(APP_ENV) ./db/scripts/migrate.sh status

migrate-version: ## Show current version
	APP_ENV=$(APP_ENV) ./db/scripts/migrate.sh version

migrate-create: ## Create new migration pair (make migrate-create NAME=xxx)
	@test -n "$(NAME)" || (echo "❌ Usage: make migrate-create NAME=add_something" && exit 1)
	./db/scripts/migrate.sh create $(NAME)

migrate-prod: ## Run migrations in production (via Docker)
	$(COMPOSE_PROD) --profile migrate up

# ══════════════════════════════════════════════════════════════
# MANUAL BACKUPS (R2)
# ══════════════════════════════════════════════════════════════

backup: ## Push manual DB dump to R2
	./db/scripts/backup_to_r2.sh

restore: ## Restore DB dump from R2
	./db/scripts/restore_from_r2.sh

admin-create: ## Create initial admin user (SQL based)
	@chmod +x ./db/scripts/create_admin.sh
	./db/scripts/create_admin.sh

demo-user: ## Create dummy user (demo@upraizo.com / demo@123)
	@chmod +x ./db/scripts/create_demo_user.sh
	./db/scripts/create_demo_user.sh

# ══════════════════════════════════════════════════════════════
# SETUP & HEALTH
# ══════════════════════════════════════════════════════════════

health: ## Full health check (DB + PgBouncer)
	APP_ENV=$(APP_ENV) ./db/scripts/health-check.sh


# ══════════════════════════════════════════════════════════════
# PRODUCTION MANAGEMENT (Modular)
# ══════════════════════════════════════════════════════════════

# ── VM Infrastructure ──
deploy-setup: ## Run VM setup script (Docker, firewall, swap)
	@chmod +x deploy/setup-vm.sh
	sudo bash deploy/setup-vm.sh

deploy-network: ## Create shared Docker network
	@docker network create upraizo_shared 2>/dev/null && echo "✅ upraizo_shared created" || echo "ℹ️  upraizo_shared already exists"

# ── Prod Server ──
prod-server-up: ## Start Prod Go Server
	@test -f server/.env.prod || (echo "❌ server/.env.prod not found!" && exit 1)
	@make deploy-network
	$(COMPOSE_SRV) up -d server
	@echo "✅ Prod Server started"

prod-server-build: ## Build and Start Prod Go Server
	$(COMPOSE_SRV) build server
	$(COMPOSE_SRV) up -d server
	@echo "✅ Prod Server built and started"

prod-server-down: ## Stop Prod Go Server
	$(COMPOSE_SRV) stop server

prod-server-logs: ## Tail Prod Go Server logs
	$(COMPOSE_SRV) logs -f server

# ── Prod Redis ──
prod-redis-up: ## Start Prod Redis
	@make deploy-network
	$(COMPOSE_SRV) up -d redis
	@echo "✅ Prod Redis started"

prod-redis-down: ## Stop Prod Redis
	$(COMPOSE_SRV) stop redis

prod-redis-logs: ## Tail Prod Redis logs
	$(COMPOSE_SRV) logs -f redis

# ── Prod RabbitMQ ──
prod-rmq-up: ## Start Prod RabbitMQ
	@make deploy-network
	$(COMPOSE_SRV) up -d rabbitmq
	@echo "✅ Prod RabbitMQ started"

prod-rmq-down: ## Stop Prod RabbitMQ
	$(COMPOSE_SRV) stop rabbitmq

prod-rmq-logs: ## Tail Prod RabbitMQ logs
	$(COMPOSE_SRV) logs -f rabbitmq

# ── Prod Nginx ──
prod-nginx-up: ## Start Prod Nginx
	@test -f server/nginx/ssl/origin.pem || (echo "❌ SSL cert not found!" && exit 1)
	@make deploy-network
	$(COMPOSE_SRV) up -d nginx
	@echo "✅ Prod Nginx started"

prod-nginx-down: ## Stop Prod Nginx
	$(COMPOSE_SRV) stop nginx

prod-nginx-logs: ## Tail Prod Nginx logs
	$(COMPOSE_SRV) logs -f nginx

prod-nginx-reload: ## Reload Nginx config (zero downtime)
	$(COMPOSE_SRV) exec nginx nginx -s reload
	@echo "✅ Nginx config reloaded"

# ── Global Prod Commands ──
prod-all-up: ## Start all services (Server, RMQ, Redis, Nginx)
	$(COMPOSE_SRV) up -d
	@echo "✅ All production services are up"

prod-all-down: ## Stop all production services
	$(COMPOSE_SRV) down
	@echo "✅ All production services stopped"

prod-status: ## Check status of all production containers
	@echo "── DB Stack ──"
	@$(COMPOSE_PROD) ps
	@echo "\n── Services Stack ──"
	@$(COMPOSE_SRV) ps
	@echo "\n── Resource Usage ──"
	@docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}"