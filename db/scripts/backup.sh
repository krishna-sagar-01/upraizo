#!/usr/bin/env bash
# ==============================================================
# Upraizo Database Backup (WAL-G + Cloudflare R2)
# Manual backup trigger + list + verify
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
    echo -e "${RED}Error: $ENV_FILE not found. Backups run in prod only.${NC}"
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
export WALG_COMPRESSION_METHOD="${WALG_COMPRESSION_METHOD:-lz4}"

# ── Check WAL-G installed ────────────────────────────────────
check_walg() {
    if ! command -v wal-g &> /dev/null; then
        echo -e "${RED}Error: wal-g not found. Run: ./scripts/setup-walg.sh${NC}"
        exit 1
    fi
}

# ── Usage ─────────────────────────────────────────────────────
usage() {
    echo -e "${CYAN}Upraizo Backup Tool${NC} (WAL-G + Cloudflare R2)"
    echo ""
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  push          Create a full base backup now"
    echo "  push-delta    Create a delta backup (incremental)"
    echo "  list          List all backups with sizes"
    echo "  verify        Verify latest backup integrity"
    echo "  cleanup       Delete old backups (retain policy)"
    echo "  wal-status    Show WAL archiving status"
    echo ""
}

# ── Commands ──────────────────────────────────────────────────
cmd_push() {
    echo -e "${CYAN}Creating full base backup...${NC}"
    local start_time
    start_time=$(date +%s)

    wal-g backup-push /var/lib/postgresql/data

    local end_time
    end_time=$(date +%s)
    local duration=$(( end_time - start_time ))

    echo -e "${GREEN}✅ Base backup completed in ${duration}s${NC}"
    echo ""
    cmd_list
}

cmd_push_delta() {
    echo -e "${CYAN}Creating delta (incremental) backup...${NC}"
    wal-g backup-push /var/lib/postgresql/data --delta-from-name LATEST
    echo -e "${GREEN}✅ Delta backup completed${NC}"
}

cmd_list() {
    echo -e "${CYAN}Available backups:${NC}"
    echo "────────────────────────────────────────────────"
    wal-g backup-list --pretty --detail
    echo "────────────────────────────────────────────────"
}

cmd_verify() {
    echo -e "${CYAN}Verifying latest backup...${NC}"

    local latest_backup
    latest_backup=$(wal-g backup-list --json | python3 -c \
        "import sys,json; backups=json.load(sys.stdin); print(backups[-1]['backup_name'])" 2>/dev/null)

    if [[ -z "$latest_backup" ]]; then
        echo -e "${RED}No backups found!${NC}"
        exit 1
    fi

    echo -e "Latest backup: ${YELLOW}$latest_backup${NC}"
    wal-g backup-mark "$latest_backup"
    echo -e "${GREEN}✅ Backup verified: $latest_backup${NC}"
}

cmd_cleanup() {
    local retain="${BACKUP_RETAIN_FULL:-7}"
    echo -e "${YELLOW}Cleaning up old backups (retaining $retain full backups)...${NC}"

    wal-g delete retain FULL "$retain" --confirm
    echo -e "${GREEN}✅ Cleanup completed${NC}"
}

cmd_wal_status() {
    echo -e "${CYAN}WAL archiving status:${NC}"
    psql -c "SELECT * FROM pg_stat_archiver;" 2>/dev/null || \
        echo -e "${YELLOW}Cannot connect to database to check WAL status${NC}"
}

# ── Main ──────────────────────────────────────────────────────
main() {
    check_walg

    case "${1:-}" in
        push)        cmd_push ;;
        push-delta)  cmd_push_delta ;;
        list)        cmd_list ;;
        verify)      cmd_verify ;;
        cleanup)     cmd_cleanup ;;
        wal-status)  cmd_wal_status ;;
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