-- ============================================================
-- Migration  : 012_create_platform_stats_table.down.sql
-- ============================================================

DROP TRIGGER IF EXISTS trg_users_stats_sync ON users;
DROP FUNCTION IF EXISTS fn_update_platform_stats();
DROP TABLE IF EXISTS platform_stats;
