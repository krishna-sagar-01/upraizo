#!/bin/bash
# ==============================================================
# Upraizo — VM Setup Script (Hetzner CAX11)
# ==============================================================
# Run as root: bash setup-vm.sh
# ==============================================================

set -euo pipefail

echo "=============================================="
echo "  Upraizo VM Setup — Hetzner CAX11"
echo "=============================================="

# ── 1. System Update ─────────────────────────────────────────
echo "[1/6] Updating system..."
apt-get update -qq && apt-get upgrade -y -qq

# ── 2. Install Docker ────────────────────────────────────────
echo "[2/6] Installing Docker..."
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
    echo "Docker installed ✓"
else
    echo "Docker already installed ✓"
fi

# ── 3. Firewall (UFW) ────────────────────────────────────────
echo "[3/6] Configuring firewall..."
apt-get install -y -qq ufw
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp    # SSH
ufw allow 80/tcp    # HTTP (Cloudflare → Nginx)
ufw allow 443/tcp   # HTTPS (Cloudflare → Nginx)
echo "y" | ufw enable
echo "Firewall configured ✓"

# ── 4. Swap (2 GB) ───────────────────────────────────────────
echo "[4/6] Setting up 2GB swap..."
if [ ! -f /swapfile ]; then
    fallocate -l 2G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
    echo "Swap configured ✓"
else
    echo "Swap already exists ✓"
fi

# ── 5. Sysctl Tunings ────────────────────────────────────────
echo "[5/6] Applying sysctl tunings..."
cat >> /etc/sysctl.conf << 'EOF'

# -- Upraizo Production Tunings --
vm.swappiness=10
vm.overcommit_memory=1
net.core.somaxconn=1024
net.ipv4.tcp_max_syn_backlog=1024
net.ipv4.ip_local_port_range=1024 65535
net.ipv4.tcp_tw_reuse=1
fs.file-max=65535
EOF
sysctl -p
echo "Sysctl tuned ✓"

# ── 6. Docker Network ────────────────────────────────────────
echo "[6/6] Creating Docker networks..."
docker network create upraizo_shared 2>/dev/null || echo "upraizo_shared already exists"
echo "Docker networks ready ✓"

echo ""
echo "=============================================="
echo "  VM Setup Complete!"
echo "=============================================="
echo ""
echo "Next steps:"
echo "  1. Clone repo:  git clone <repo-url> /opt/upraizo"
echo "  2. Setup SSL:   mkdir -p /opt/upraizo/server/nginx/ssl"
echo "  3. Add certs:   Copy origin.pem and origin-key.pem"
echo "  4. Fill .env:   Edit .env.prod and db/.env files"
echo "  5. Start DB:    cd /opt/upraizo/db/docker/prod && docker compose up -d"
echo "  6. Migrate:     docker compose --profile migrate up"
echo "  7. Start App:   cd /opt/upraizo/server && docker compose -f docker-compose.prod.yml up -d"
echo ""
