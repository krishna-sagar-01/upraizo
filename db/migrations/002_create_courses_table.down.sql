-- ============================================================
-- Migration  : 002_create_courses_table.down
-- ============================================================

-- Drop trigger if it exists (used for updating updated_at automatically)
DROP TRIGGER IF EXISTS trg_courses_updated_at ON courses;

-- Drop courses table if it exists
DROP TABLE IF EXISTS courses;