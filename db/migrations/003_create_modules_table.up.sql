-- ============================================================
-- Migration  : 003_create_modules_table
-- ============================================================

CREATE TABLE modules (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id   UUID        NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    title       TEXT        NOT NULL,
    order_index INTEGER     NOT NULL DEFAULT 0 CHECK (order_index >= 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_modules_course_order UNIQUE (course_id, order_index)
);

CREATE INDEX idx_modules_course_order
    ON modules (course_id, order_index ASC);

CREATE TRIGGER trg_modules_updated_at
    BEFORE UPDATE ON modules
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- ── Comments ─────────────────────────────────────────────────
COMMENT ON TABLE  modules             IS 'Sections/modules within a course';
COMMENT ON COLUMN modules.order_index IS 'Module position within the course, starts from 0';