-- ================================================
-- 000_enable_extensions.down.sql
-- ================================================

DROP EXTENSION IF EXISTS "citext";
DROP EXTENSION IF EXISTS "pg_trgm";

-- ================================================
-- SHARED UPDATE FUNCTIONS
-- ================================================
DROP FUNCTION IF EXISTS fn_set_updated_at();