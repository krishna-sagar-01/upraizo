#!/bin/bash
# ==============================================================================
# Upraizo Admin Creation Script (SQL Based)
# Usage: ./db/scripts/create_admin.sh
# ==============================================================================

# ── Configuration ──────────────────────────────────────────────
# Load DB environment variables if they exist
ENV_FILE="db/docker/dev/.env"
if [ -f "$ENV_FILE" ]; then
    export $(grep -v '^#' "$ENV_FILE" | xargs)
fi

DB_NAME=${POSTGRES_DB:-upraizo_db}
DB_USER=${POSTGRES_USER:-upraizo_admin}
DB_HOST=${POSTGRES_HOST:-localhost}
DB_PORT=${POSTGRES_PORT:-5432}
DB_PASS=${POSTGRES_PASSWORD}

# ── Admin Details ─────────────────────────────────────────────
# EDIT THESE FIELDS
NAME="Kunal"
EMAIL="kunalsingh75828@gmail.com"
PHONE="+911234567890"

# HASHED VALUES (Bcrypt Cost 12)
# Tip: Use 'go run cmd/admin/main.go' to generate these or enter manually.
# Placeholder hash for "admin123" is provided below.
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

if [ "$(docker ps -q -f name=upraizo_db_dev)" ]; then
    echo "💾 Using Docker container to run SQL..."
    docker exec -e PGPASSWORD=$DB_PASS upraizo_db_dev psql -h localhost -U $DB_USER -d $DB_NAME -c "$SQL_QUERY"
else
    echo "🌐 Using local psql..."
    PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "$SQL_QUERY"
fi


if [ $? -eq 0 ]; then
    echo "✅ Admin '$NAME' created successfully (or already exists)."
else
    echo "❌ Failed to create admin."
    exit 1
fi
