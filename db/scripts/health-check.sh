#!/usr/bin/env bash
# ==============================================================
# Upraizo Database Health Check
# Checks connectivity for Postgres and PgBouncer
# ==============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# Load env based on APP_ENV
ENV_TYPE="${APP_ENV:-dev}"
if [[ "$ENV_TYPE" == "prod" ]]; then
    ENV_FILE="$PROJECT_ROOT/docker/prod/.env"
else
    ENV_FILE="$PROJECT_ROOT/docker/dev/.env"
fi

if [[ ! -f "$ENV_FILE" ]]; then
    echo -e "${RED}Error: $ENV_FILE not found.${NC}"
    exit 1
fi

source "$ENV_FILE"

check_postgres() {
    echo -ne "${CYAN}Checking PostgreSQL (${ENV_TYPE})... ${NC}"
    if pg_isready -h localhost -p 5432 -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" &> /dev/null; then
        echo -e "${GREEN}ONLINE${NC}"
    else
        echo -e "${RED}OFFLINE (or inaccessible)${NC}"
    fi
}

check_pgbouncer() {
    if [[ "$ENV_TYPE" == "prod" ]]; then
        echo -ne "${CYAN}Checking PgBouncer (Port 6432)... ${NC}"
        if pg_isready -h localhost -p 6432 -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" &> /dev/null; then
            echo -e "${GREEN}ONLINE${NC}"
        else
            echo -e "${YELLOW}OFFLINE (Normal if not running in production mode)${NC}"
        fi
    fi
}

echo -e "${CYAN}Upraizo Database Health Diagnostics${NC}"
echo "────────────────────────────────────────────────"
check_postgres
check_pgbouncer
echo "────────────────────────────────────────────────"