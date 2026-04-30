-- ============================================================
-- Migration  : 003_create_modules_table.down
-- ============================================================

-- Drop trigger if it exists (used for updating updated_at automatically)
DROP TRIGGER IF EXISTS trg_modules_updated_at ON modules;

-- Drop modules table if it exists
DROP TABLE IF EXISTS modules;