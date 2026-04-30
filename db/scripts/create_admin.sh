#!/bin/bash
# ==============================================================================
# Upraizo Admin Creation Script (SQL Based)
# Usage: ./db/scripts/create_admin.sh
# ==============================================================================

# ── Configuration ──────────────────────────────────────────────
# Load DB environment variables (Prod first, then Dev)
if [ -f "db/docker/prod/.env" ]; then
    ENV_FILE="db/docker/prod/.env"
    CONTAINER_NAME="upraizo-db-prod"
elif [ -f "db/docker/dev/.env" ]; then
    ENV_FILE="db/docker/dev/.env"
    CONTAINER_NAME="upraizo-db-dev"
else
    echo "❌ No environment file found!"
    exit 1
fi

export $(grep -v '^#' "$ENV_FILE" | xargs)

DB_NAME=${POSTGRES_DB:-upraizo_db}
DB_USER=${POSTGRES_USER:-upraizo_admin}
DB_PASS=${POSTGRES_PASSWORD}

# ── Admin Details ─────────────────────────────────────────────
NAME="Krishna"
EMAIL="Imexplorerkrishna@gmail.com"
PHONE="+918433002994"

# HASHED VALUES (Bcrypt Cost 12)
PASS_HASH='$2a$12$ZfslJkjlI505OY5W9gy/d.ZyqWoBMqqSZc5FKXopjZZKsEVzvkrR.'
SECRET_HASH='$2a$12$ZfslJkjlI505OY5W9gy/d.ZyqWoBMqqSZc5FKXopjZZKsEVzvkrR.'

# ── Execution ─────────────────────────────────────────────────
echo "🚀 Creating admin: $NAME ($EMAIL)..."

SQL_QUERY="INSERT INTO admins (name, email, phone, password_hash, secret_key_hash) \
VALUES ('$NAME', '$EMAIL', '$PHONE', '$PASS_HASH', '$SECRET_HASH') \
ON CONFLICT (email) DO UPDATE SET \
    password_hash = EXCLUDED.password_hash, \
    secret_key_hash = EXCLUDED.secret_key_hash, \
    name = EXCLUDED.name, \
    phone = EXCLUDED.phone;"

if [ "$(docker ps -q -f name=$CONTAINER_NAME)" ]; then
    echo "💾 Using Docker container ($CONTAINER_NAME) to run SQL..."
    docker exec -e PGPASSWORD=$DB_PASS $CONTAINER_NAME psql -U $DB_USER -d $DB_NAME -c "$SQL_QUERY"
else
    echo "❌ Database container ($CONTAINER_NAME) is not running!"
    exit 1
fi

if [ $? -eq 0 ]; then
    echo "✅ Admin '$NAME' created successfully."
else
    echo "❌ Failed to create admin."
    exit 1
fi
