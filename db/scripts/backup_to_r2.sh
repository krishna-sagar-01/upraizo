#!/usr/bin/env bash
# ==============================================================
# Upraizo Simple Backup to R2 (Using pg_dump + aws-cli)
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

# Config
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_NAME="upraizo_prod_${TIMESTAMP}.sql.gz"
TEMP_DIR="/tmp/upraizo_backups"
mkdir -p "$TEMP_DIR"

# S3 / R2 Config
export AWS_ACCESS_KEY_ID="${R2_ACCESS_KEY_ID}"
export AWS_SECRET_ACCESS_KEY="${R2_SECRET_ACCESS_KEY}"
R2_URL="s3://${R2_BUCKET}/manual_dumps/${BACKUP_NAME}"

echo "🚀 Starting backup of ${POSTGRES_DB}..."

# 1. Create dump from docker container
docker exec upraizo-db-prod pg_dump -U "${POSTGRES_USER}" "${POSTGRES_DB}" | gzip > "${TEMP_DIR}/${BACKUP_NAME}"

echo "📦 Backup created locally: ${TEMP_DIR}/${BACKUP_NAME}"

# 2. Upload to R2
echo "☁️ Uploading to R2..."
aws s3 cp "${TEMP_DIR}/${BACKUP_NAME}" "${R2_URL}" --endpoint-url "${R2_ENDPOINT}" --region auto

echo "✅ Backup successfully uploaded to: ${R2_URL}"

# 3. Cleanup temp file
rm "${TEMP_DIR}/${BACKUP_NAME}"
echo "✨ Local cleanup done."
