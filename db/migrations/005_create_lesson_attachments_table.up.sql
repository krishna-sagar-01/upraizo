-- ============================================================
-- Migration  : 005_create_lesson_attachments_table
-- ============================================================

CREATE TABLE lesson_attachments (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id   UUID        NOT NULL REFERENCES lessons (id) ON DELETE CASCADE,
    title       TEXT        NOT NULL,
    file_key    TEXT        NOT NULL,
    file_size   BIGINT      NOT NULL CHECK (file_size > 0),
    mime_type   TEXT        NOT NULL DEFAULT 'application/pdf',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_lesson_attachments_key UNIQUE (file_key),

    -- MIME validation
    CONSTRAINT chk_mime_type CHECK (
        mime_type ~ '^[a-z]+/[a-z0-9.+-]+$'
    )
);

CREATE INDEX idx_lesson_attachments_lesson_id
    ON lesson_attachments (lesson_id);

-- ── Comments ─────────────────────────────────────────────────
COMMENT ON TABLE  lesson_attachments           IS 'Files attached to a lesson (PDFs, notes, etc.)';
COMMENT ON COLUMN lesson_attachments.file_key  IS 'Cloudflare R2 object key';
COMMENT ON COLUMN lesson_attachments.file_size IS 'File size in bytes';
COMMENT ON COLUMN lesson_attachments.mime_type IS 'MIME type (e.g. application/pdf, application/zip)';
