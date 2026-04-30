-- ============================================================
-- Migration  : 004_create_lessons_table.down
-- ============================================================

-- Drop trigger if it exists (used for updating updated_at automatically)
DROP TRIGGER IF EXISTS trg_lessons_updated_at ON lessons;

-- Drop lessons table if it exists
DROP TABLE IF EXISTS lessons;

-- Drop enum type if it exists
DROP TYPE IF EXISTS video_status;