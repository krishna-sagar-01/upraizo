#!/bin/bash
# ==============================================================================
# Upraizo Demo User Creation Script
# Usage: ./db/scripts/create_demo_user.sh
# ==============================================================================

# ── Configuration ──────────────────────────────────────────────
# Load DB environment variables if they exist
ENV_FILE="db/docker/prod/.env"
if [ -f "$ENV_FILE" ]; then
    export $(grep -v '^#' "$ENV_FILE" | xargs)
fi

DB_NAME=${POSTGRES_DB:-upraizo_db}
DB_USER=${POSTGRES_USER:-upraizo_admin}
DB_PASS=${POSTGRES_PASSWORD}

# ── User Details ─────────────────────────────────────────────
NAME="Demo User"
EMAIL="demo@upraizo.com"
# Password: demo@123
PASS_HASH='$2b$12$4nVqojW1jsMnuAMILhaGduw2jlk34cx4FNKDK79o0F0FC2VuHwrua'

# ── Execution ─────────────────────────────────────────────────
echo "🚀 Creating demo user: $NAME ($EMAIL)..."

SQL_QUERY="INSERT INTO users (name, email, password_hash, status, is_verified, verified_at) \
VALUES ('$NAME', '$EMAIL', '$PASS_HASH', 'active', TRUE, NOW()) \
ON CONFLICT (email) DO UPDATE SET \
    password_hash = EXCLUDED.password_hash, \
    status = EXCLUDED.status, \
    is_verified = EXCLUDED.is_verified, \
    verified_at = EXCLUDED.verified_at, \
    name = EXCLUDED.name;"

if [ "$(docker ps -q -f name=upraizo-db-prod)" ]; then
    echo "💾 Using Docker container (upraizo-db-prod) to run SQL..."
    docker exec -e PGPASSWORD=$DB_PASS upraizo-db-prod psql -U $DB_USER -d $DB_NAME -c "$SQL_QUERY"
else
    echo "❌ Production DB container (upraizo-db-prod) not found!"
    exit 1
fi

if [ $? -eq 0 ]; then
    echo "✅ Demo user created successfully (or already exists)."
else
    echo "❌ Failed to create demo user."
    exit 1
fi
