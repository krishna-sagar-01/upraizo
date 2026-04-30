#!/usr/bin/env bash
# ==============================================================
# Upraizo Simple Restore from R2 (Using aws-cli + pg_restore)
# ==============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load production env
ENV_FILE="$PROJECT_ROOT/docker/prod/.env"
if [[ ! -f "$ENV_FILE" ]]; then
    echo "❌ Error: $ENV_FILE not found."
    exit 1
fi
source "$ENV_FILE"

# S3 / R2 Config
export AWS_ACCESS_KEY_ID="${R2_ACCESS_KEY_ID}"
export AWS_SECRET_ACCESS_KEY="${R2_SECRET_ACCESS_KEY}"
TEMP_DIR="/tmp/upraizo_restores"
mkdir -p "$TEMP_DIR"

echo "🔍 Fetching available backups from R2..."
aws s3 ls "s3://${R2_BUCKET}/manual_dumps/" --endpoint-url "${R2_ENDPOINT}" --region auto || { echo "❌ No backups found."; exit 1; }

echo ""
echo -n "👉 Enter the filename to restore (e.g., upraizo_prod_20260429_215806.sql.gz): "
read -r FILENAME

if [[ -z "$FILENAME" ]]; then
    echo "❌ No filename entered. Aborting."
    exit 1
fi

LOCAL_FILE="${TEMP_DIR}/${FILENAME}"

# 1. Download from R2
echo "⏳ Downloading ${FILENAME} from R2..."
aws s3 cp "s3://${R2_BUCKET}/manual_dumps/${FILENAME}" "$LOCAL_FILE" --endpoint-url "${R2_ENDPOINT}" --region auto

# 2. Restore to container
echo "🔄 Restoring to ${POSTGRES_DB} (This will overwrite current data)..."
echo "⚠️  Wait, dropping existing connections..."

# Drop and Recreate DB inside container for a clean restore
docker exec -it upraizo-db-prod psql -U "${POSTGRES_USER}" -d postgres -c "DROP DATABASE IF EXISTS ${POSTGRES_DB};"
docker exec -it upraizo-db-prod psql -U "${POSTGRES_USER}" -d postgres -c "CREATE DATABASE ${POSTGRES_DB} OWNER ${POSTGRES_USER};"

gunzip -c "$LOCAL_FILE" | docker exec -i upraizo-db-prod psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}"

echo "✅ Restore completed successfully!"

# 3. Cleanup
rm "$LOCAL_FILE"
echo "✨ Cleanup done."
