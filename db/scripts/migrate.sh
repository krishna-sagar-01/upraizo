#!/usr/bin/env bash
# ==============================================================
# Upraizo Migration Runner
# Wraps golang-migrate CLI for dev/prod environments
# ==============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MIGRATIONS_DIR="$PROJECT_ROOT/migrations"

# ── Colors ────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# ── Determine Environment ────────────────────────────────────
APP_ENV="${APP_ENV:-dev}"
ENV_FILE="$PROJECT_ROOT/docker/$APP_ENV/.env"

if [[ ! -f "$ENV_FILE" ]]; then
    echo -e "${RED}Error: $ENV_FILE not found.${NC}"
    echo -e "Run: ${CYAN}cp $PROJECT_ROOT/docker/$APP_ENV/.env.example $ENV_FILE${NC}"
    exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"

# ── Build Database URL ────────────────────────────────────────
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${POSTGRES_PORT:-5432}"
DB_NAME="${POSTGRES_DB:-forge_db}"
DB_USER="${POSTGRES_USER:-forge_admin}"
DB_PASS="${POSTGRES_PASSWORD}"
DB_SSL="${DB_SSLMODE:-disable}"

DATABASE_URL="postgres://${DB_USER}:${DB_PASS}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL}"

# ── Check golang-migrate installed ────────────────────────────
check_migrate() {
    if ! command -v migrate &> /dev/null; then
        echo -e "${YELLOW}golang-migrate not found. Installing...${NC}"
        curl -sL https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz \
            | tar -xz -C /usr/local/bin/ migrate
        chmod +x /usr/local/bin/migrate
        echo -e "${GREEN}Installed golang-migrate v4.17.0${NC}"
    fi
}

# ── Usage ─────────────────────────────────────────────────────
usage() {
    echo -e "${CYAN}Upraizo Migration Tool${NC} (env: ${YELLOW}$APP_ENV${NC})"
    echo ""
    echo "Usage: $0 <command> [args]"
    echo ""
    echo "Commands:"
    echo "  up              Run all pending UP migrations"
    echo "  up N            Run N UP migrations"
    echo "  down N          Rollback N migrations"
    echo "  goto VERSION    Migrate to specific version"
    echo "  force VERSION   Force set version (fix dirty state)"
    echo "  drop            Drop everything (dev only!)"
    echo "  version         Show current migration version"
    echo "  status          Show current version + pending count"
    echo "  create NAME     Create new migration pair"
    echo ""
    echo "Environment:"
    echo "  APP_ENV=dev|prod   (default: dev)"
    echo "  DB_HOST=hostname   (default: localhost)"
    echo ""
    echo "Examples:"
    echo "  $0 up                         # Run all pending"
    echo "  $0 up 5                       # Run next 5"
    echo "  $0 down 1                     # Rollback last one"
    echo "  $0 goto 42                    # Go to version 42"
    echo "  APP_ENV=prod $0 up            # Run in production"
    echo "  $0 create add_audit_columns   # Create new pair"
}

# ── Confirm for production ────────────────────────────────────
confirm_prod() {
    if [[ "$APP_ENV" == "prod" ]]; then
        echo -e "${RED}⚠️  WARNING: You are running migrations on PRODUCTION!${NC}"
        echo -e "Database: ${YELLOW}${DB_HOST}:${DB_PORT}/${DB_NAME}${NC}"
        read -rp "Type 'yes' to confirm: " confirm
        if [[ "$confirm" != "yes" ]]; then
            echo "Aborted."
            exit 1
        fi
    fi
}

# ── Commands ──────────────────────────────────────────────────
cmd_up() {
    local steps="${1:-}"
    confirm_prod
    echo -e "${CYAN}Running UP migrations...${NC} (env: $APP_ENV)"

    if [[ -n "$steps" ]]; then
        migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" up "$steps"
    else
        migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" up
    fi

    echo -e "${GREEN}✅ Migrations completed successfully.${NC}"
}

cmd_down() {
    local steps="${1:?Error: specify number of migrations to rollback}"

    if [[ "$APP_ENV" == "prod" ]]; then
        echo -e "${RED}⚠️  DANGER: Rolling back $steps migration(s) on PRODUCTION!${NC}"
        echo -e "Database: ${YELLOW}${DB_HOST}:${DB_PORT}/${DB_NAME}${NC}"
        read -rp "Type 'ROLLBACK' to confirm: " confirm
        if [[ "$confirm" != "ROLLBACK" ]]; then
            echo "Aborted."
            exit 1
        fi
    fi

    echo -e "${YELLOW}Rolling back $steps migration(s)...${NC}"
    migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" down "$steps"
    echo -e "${GREEN}✅ Rollback completed.${NC}"
}

cmd_goto() {
    local version="${1:?Error: specify target version}"
    confirm_prod
    echo -e "${CYAN}Migrating to version $version...${NC}"
    migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" goto "$version"
    echo -e "${GREEN}✅ Migrated to version $version.${NC}"
}

cmd_force() {
    local version="${1:?Error: specify version to force}"
    echo -e "${YELLOW}Forcing version to $version...${NC}"
    migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" force "$version"
    echo -e "${GREEN}✅ Version forced to $version.${NC}"
}

cmd_drop() {
    if [[ "$APP_ENV" == "prod" ]]; then
        echo -e "${RED}Cannot drop in production!${NC}"
        exit 1
    fi
    echo -e "${RED}Dropping ALL tables and data...${NC}"
    read -rp "Type 'DROP' to confirm: " confirm
    if [[ "$confirm" != "DROP" ]]; then
        echo "Aborted."
        exit 1
    fi
    migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" drop -f
    echo -e "${GREEN}✅ All tables dropped.${NC}"
}

cmd_version() {
    migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" version
}

cmd_status() {
    echo -e "${CYAN}Environment:${NC}  $APP_ENV"
    echo -e "${CYAN}Database:${NC}     ${DB_HOST}:${DB_PORT}/${DB_NAME}"
    echo -e "${CYAN}Version:${NC}      $(migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" version 2>&1 || true)"

    local total_migrations
    total_migrations=$(find "$MIGRATIONS_DIR" -name "*.up.sql" | wc -l)
    echo -e "${CYAN}Total files:${NC}  $total_migrations UP + $total_migrations DOWN"
}

cmd_create() {
    local name="${1:?Error: specify migration name}"
    local next_num

    # Find next migration number
    next_num=$(find "$MIGRATIONS_DIR" -name "*.up.sql" \
        | sed 's/.*\///' | sort -n | tail -1 \
        | grep -oP '^\d+' | awk '{printf "%03d", $1 + 1}')

    local up_file="$MIGRATIONS_DIR/${next_num}_${name}.up.sql"
    local down_file="$MIGRATIONS_DIR/${next_num}_${name}.down.sql"

    cat > "$up_file" <<EOF
-- ============================================================
-- Migration  : ${next_num}_${name}
-- Description: TODO
-- ============================================================


EOF

    cat > "$down_file" <<EOF
-- ============================================================
-- Migration  : ${next_num}_${name} (ROLLBACK)
-- ============================================================


EOF

    echo -e "${GREEN}Created:${NC}"
    echo "  $up_file"
    echo "  $down_file"
}

# ── Main ──────────────────────────────────────────────────────
main() {
    check_migrate

    case "${1:-}" in
        up)       cmd_up "${2:-}" ;;
        down)     cmd_down "${2:-}" ;;
        goto)     cmd_goto "${2:-}" ;;
        force)    cmd_force "${2:-}" ;;
        drop)     cmd_drop ;;
        version)  cmd_version ;;
        status)   cmd_status ;;
        create)   cmd_create "${2:-}" ;;
        -h|--help|help|"")
            usage ;;
        *)
            echo -e "${RED}Unknown command: $1${NC}"
            usage
            exit 1
            ;;
    esac
}

main "$@"