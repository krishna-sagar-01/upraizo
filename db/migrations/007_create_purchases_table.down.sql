-- ============================================================
-- Migration  : 006_create_purchases_table.down
-- ============================================================

DROP TRIGGER IF EXISTS trg_purchases_updated_at ON purchases;

DROP TABLE IF EXISTS purchases;

DROP TYPE IF EXISTS purchase_status;