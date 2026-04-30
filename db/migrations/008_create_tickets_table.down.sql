-- ============================================================
-- Migration  : 008_create_tickets_table.down
-- ============================================================

-- 1. Drop trigger first
DROP TRIGGER IF EXISTS trg_tickets_updated_at ON tickets;

-- 2. Drop the main table
DROP TABLE IF EXISTS tickets;

-- 3. Drop the custom ENUM types
DROP TYPE IF EXISTS ticket_status;
DROP TYPE IF EXISTS ticket_priority;