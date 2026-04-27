#!/usr/bin/env bash
# ==============================================================
# Upraizo WAL-G Setup Script
# Installs WAL-G and configures cron for automated backups
# ==============================================================
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

WALG_VERSION="v3.0.0"
WALG_URL="https://github.com/wal-g/wal-g/releases/download/${WALG_VERSION}/wal-g-pg-ubuntu-20.04-amd64.tar.gz"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo -e "${CYAN}╔══════════════════════════════════════╗${NC}"
echo -e "${CYAN}║   Upraizo WAL-G Setup                 ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
echo ""

# ── Step 1: Install WAL-G ────────────────────────────────────
echo -e "${CYAN}[1/4] Installing WAL-G $WALG_VERSION...${NC}"
if command -v wal-g &> /dev/null; then
    echo -e "${GREEN}WAL-G already installed: $(wal-g --version 2>/dev/null || echo 'unknown version')${NC}"
else
    curl -sL "$WALG_URL" | tar -xz -C /usr/local/bin/
    chmod +x /usr/local/bin/wal-g
    echo -e "${GREEN}✅ WAL-G installed to /usr/local/bin/wal-g${NC}"
fi
echo ""

# ── Step 2: Verify R2 credentials ────────────────────────────
echo -e "${CYAN}[2/4] Checking R2 credentials...${NC}"
ENV_FILE="$PROJECT_ROOT/docker/prod/.env"
if [[ ! -f "$ENV_FILE" ]]; then
    echo -e "${RED}Error: $ENV_FILE not found!${NC}"
    echo -e "Run: ${YELLOW}cp $PROJECT_ROOT/docker/prod/.env.example $ENV_FILE${NC}"
    exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"

# Verify required vars
for var in WALG_S3_PREFIX AWS_ENDPOINT AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY; do
    if [[ -z "${!var:-}" || "${!var}" == CHANGE_ME* ]]; then
        echo -e "${RED}Error: $var is not set or still has placeholder value!${NC}"
        echo -e "Edit ${YELLOW}$ENV_FILE${NC} and set all R2 credentials."
        exit 1
    fi
done
echo -e "${GREEN}✅ R2 credentials configured${NC}"
echo ""

# ── Step 3: Test R2 connection ────────────────────────────────
echo -e "${CYAN}[3/4] Testing R2 connection...${NC}"
export WALG_S3_PREFIX AWS_ENDPOINT AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
export AWS_REGION="${AWS_REGION:-auto}"
export AWS_S3_FORCE_PATH_STYLE=true

if wal-g backup-list 2>/dev/null; then
    echo -e "${GREEN}✅ R2 connection successful${NC}"
else
    echo -e "${YELLOW}⚠️  No backups found (expected for first setup)${NC}"
fi
echo ""

# ── Step 4: Setup cron job ────────────────────────────────────
echo -e "${CYAN}[4/4] Setting up backup cron job...${NC}"

CRON_SCRIPT="/etc/cron.d/upraizo-backup"
BACKUP_SCRIPT="$SCRIPT_DIR/backup.sh"

cat > "$CRON_SCRIPT" <<EOF
# upraizo Database Backup Schedule
# Daily base backup at 2:00 AM IST (20:30 UTC)
30 20 * * * root ${BACKUP_SCRIPT} push >> /var/log/upraizo-backup.log 2>&1

# Weekly cleanup (Sunday 3:00 AM IST)
30 21 * * 0 root ${BACKUP_SCRIPT} cleanup >> /var/log/upraizo-backup.log 2>&1
EOF

chmod 0644 "$CRON_SCRIPT"
echo -e "${GREEN}✅ Cron job created at $CRON_SCRIPT${NC}"
echo ""

# ── Summary ───────────────────────────────────────────────────
echo -e "${GREEN}╔══════════════════════════════════════╗${NC}"
echo -e "${GREEN}║   Setup Complete!                    ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════╝${NC}"
echo ""
echo -e "  WAL-G:     ${GREEN}Installed${NC}"
echo -e "  R2:        ${GREEN}Connected${NC}"
echo -e "  Cron:      ${GREEN}Daily 2:00 AM IST${NC}"
echo ""
echo -e "Next steps:"
echo -e "  1. Create first backup:  ${CYAN}./scripts/backup.sh push${NC}"
echo -e "  2. Verify backup:        ${CYAN}./scripts/backup.sh verify${NC}"
echo -e "  3. List backups:         ${CYAN}./scripts/backup.sh list${NC}"