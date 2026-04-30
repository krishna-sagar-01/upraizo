-- ============================================================
-- Migration  : 001_create_users_table.down.sql
-- ============================================================

DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
DROP TABLE   IF EXISTS users;
DROP TYPE    IF EXISTS user_status;