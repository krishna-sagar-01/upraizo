-- ============================================================
-- Migration  : 004_create_lessons_table
-- ============================================================

CREATE TYPE video_status AS ENUM ('pending', 'processing', 'ready', 'failed');

CREATE TABLE lessons (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id        UUID         NOT NULL REFERENCES modules (id) ON DELETE CASCADE,
    title            TEXT         NOT NULL,
    order_index      INTEGER      NOT NULL DEFAULT 0 CHECK (order_index >= 0),

    video_key        TEXT         NULL,
    video_status     video_status NOT NULL DEFAULT 'pending',

    duration_seconds INTEGER      NULL CHECK (duration_seconds >= 0),
    is_preview       BOOLEAN      NOT NULL DEFAULT FALSE,

    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_lessons_module_order UNIQUE (module_id, order_index),

    -- Strict video consistency
    CONSTRAINT chk_lessons_video CHECK (
        (video_key IS NULL AND video_status = 'pending')
        OR
        (video_key IS NOT NULL AND video_status IN ('processing', 'ready', 'failed'))
    )
);

CREATE INDEX idx_lessons_module_order
    ON lessons (module_id, order_index ASC);

CREATE INDEX idx_lessons_video_status
    ON lessons (video_status)
    WHERE video_status IN ('pending', 'processing');

CREATE INDEX idx_lessons_preview
    ON lessons (module_id)
    WHERE is_preview = TRUE;

CREATE TRIGGER trg_lessons_updated_at
    BEFORE UPDATE ON lessons
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- ── Comments ─────────────────────────────────────────────────
COMMENT ON TABLE  lessons                  IS 'Individual video lessons within a module';
COMMENT ON COLUMN lessons.video_key        IS 'Cloudflare R2 object key for storing the HLS video';
COMMENT ON COLUMN lessons.video_status     IS 'pending = not uploaded, processing = transcoding in progress, ready = available for viewing, failed = transcoding failed';
COMMENT ON COLUMN lessons.duration_seconds IS 'Video duration, set after FFmpeg transcoding';
COMMENT ON COLUMN lessons.is_preview       IS 'TRUE = accessible without purchase (free preview)';

