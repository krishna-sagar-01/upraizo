#!/usr/bin/env bash
# ==============================================================
# Upraizo Database Restore (WAL-G + Cloudflare R2)
# Restore from backup — supports PITR
# ==============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# ── Load prod env ─────────────────────────────────────────────
ENV_FILE="$PROJECT_ROOT/docker/prod/.env"
if [[ ! -f "$ENV_FILE" ]]; then
    echo -e "${RED}Error: $ENV_FILE not found.${NC}"
    exit 1
fi
# shellcheck disable=SC1090
source "$ENV_FILE"

# ── WAL-G environment ────────────────────────────────────────
export PGHOST="${PGHOST:-localhost}"
export PGPORT="${POSTGRES_PORT:-5432}"
export PGUSER="${POSTGRES_USER}"
export PGPASSWORD="${POSTGRES_PASSWORD}"
export PGDATABASE="${POSTGRES_DB}"
export WALG_S3_PREFIX="${WALG_S3_PREFIX}"
export AWS_ENDPOINT="${AWS_ENDPOINT}"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}"
export AWS_REGION="${AWS_REGION:-auto}"
export AWS_S3_FORCE_PATH_STYLE=true

PGDATA="/var/lib/postgresql/data"

# ── Usage ─────────────────────────────────────────────────────
usage() {
    echo -e "${CYAN}Upraizo Restore Tool${NC} (WAL-G PITR)"
    echo ""
    echo "Usage: $0 <command> [args]"
    echo ""
    echo "Commands:"
    echo "  latest                     Restore from latest backup"
    echo "  pitr TIMESTAMP             Restore to specific point in time"
    echo "  backup BACKUP_NAME         Restore specific backup"
    echo "  list                       List available backups"
    echo ""
    echo "PITR Examples:"
    echo "  $0 pitr '2026-03-05 10:30:00+05:30'"
    echo "  $0 pitr '2026-03-05 04:00:00 UTC'"
    echo ""
    echo -e "${RED}⚠️  WARNING: Restore is DESTRUCTIVE — it replaces ALL data!${NC}"
}

# ── Stop PostgreSQL ───────────────────────────────────────────
stop_postgres() {
    echo -e "${YELLOW}Stopping PostgreSQL...${NC}"
    docker compose -f "$PROJECT_ROOT/docker/prod/docker-compose.yml" stop db
    sleep 3
}

# ── Start PostgreSQL ──────────────────────────────────────────
start_postgres() {
    echo -e "${CYAN}Starting PostgreSQL...${NC}"
    docker compose -f "$PROJECT_ROOT/docker/prod/docker-compose.yml" start db
    sleep 5
    echo -e "${GREEN}PostgreSQL started.${NC}"
}

# ── Confirm ───────────────────────────────────────────────────
confirm_restore() {
    local target="$1"
    echo ""
    echo -e "${RED}╔══════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║  ⚠️   DESTRUCTIVE OPERATION — DATA WILL BE LOST  ║${NC}"
    echo -e "${RED}╚══════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "  Restore target:  ${YELLOW}$target${NC}"
    echo -e "  Database:        ${YELLOW}${PGHOST}:${PGPORT}/${PGDATABASE}${NC}"
    echo ""
    echo -e "This will:"
    echo -e "  1. Stop PostgreSQL"
    echo -e "  2. Wipe the current data directory"
    echo -e "  3. Restore from backup"
    echo -e "  4. Restart PostgreSQL"
    echo ""
    read -rp "Type 'RESTORE' to confirm: " confirm
    if [[ "$confirm" != "RESTORE" ]]; then
        echo "Aborted."
        exit 1
    fi
}

# ── Restore Commands ──────────────────────────────────────────
cmd_latest() {
    confirm_restore "LATEST backup"
    stop_postgres

    echo -e "${CYAN}Restoring from latest backup...${NC}"
    wal-g backup-fetch "$PGDATA" LATEST

    # Create recovery signal file for PG 12+
    touch "$PGDATA/recovery.signal"
    cat >> "$PGDATA/postgresql.auto.conf" <<EOF
restore_command = 'wal-g wal-fetch %f %p'
recovery_target_action = 'promote'
EOF

    start_postgres
    echo -e "${GREEN}✅ Restore from latest backup completed.${NC}"
    echo -e "${YELLOW}PostgreSQL will replay WAL segments automatically.${NC}"
}

cmd_pitr() {
    local target_time="${1:?Error: specify target timestamp}"
    confirm_restore "PITR → $target_time"
    stop_postgres

    echo -e "${CYAN}Restoring to point-in-time: $target_time${NC}"
    wal-g backup-fetch "$PGDATA" LATEST

    # PITR recovery config
    touch "$PGDATA/recovery.signal"
    cat >> "$PGDATA/postgresql.auto.conf" <<EOF
restore_command = 'wal-g wal-fetch %f %p'
recovery_target_time = '$target_time'
recovery_target_action = 'promote'
EOF

    start_postgres
    echo -e "${GREEN}✅ PITR restore completed. Target: $target_time${NC}"
    echo -e "${YELLOW}PostgreSQL will replay WAL until target time.${NC}"
}

cmd_backup() {
    local backup_name="${1:?Error: specify backup name}"
    confirm_restore "backup: $backup_name"
    stop_postgres

    echo -e "${CYAN}Restoring backup: $backup_name${NC}"
    wal-g backup-fetch "$PGDATA" "$backup_name"

    touch "$PGDATA/recovery.signal"
    cat >> "$PGDATA/postgresql.auto.conf" <<EOF
restore_command = 'wal-g wal-fetch %f %p'
recovery_target_action = 'promote'
EOF

    start_postgres
    echo -e "${GREEN}✅ Restored from backup: $backup_name${NC}"
}

cmd_list() {
    echo -e "${CYAN}Available backups:${NC}"
    wal-g backup-list --pretty --detail
}

# ── Main ──────────────────────────────────────────────────────
main() {
    if ! command -v wal-g &> /dev/null; then
        echo -e "${RED}Error: wal-g not found. Run: ./scripts/setup-walg.sh${NC}"
        exit 1
    fi

    case "${1:-}" in
        latest)   cmd_latest ;;
        pitr)     cmd_pitr "${2:-}" ;;
        backup)   cmd_backup "${2:-}" ;;
        list)     cmd_list ;;
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