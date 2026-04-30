-- ============================================================
-- Migration  : 012_create_ebooks_table (ROLLBACK)
-- ============================================================

DROP TRIGGER IF EXISTS set_ebooks_updated_at ON ebooks;
DROP TABLE IF EXISTS ebooks;
