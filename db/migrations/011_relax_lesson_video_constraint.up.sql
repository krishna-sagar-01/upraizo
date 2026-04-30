-- Migration: 014_relax_lesson_video_constraint
-- Description: Allow video_key to be NULL when video_status is 'processing' or 'failed'.

BEGIN;

-- 1. Drop existing constraint
ALTER TABLE lessons DROP CONSTRAINT IF EXISTS chk_lessons_video;

-- 2. Add relaxed constraint
-- Logic:
-- - If status is 'pending', key MUST be NULL.
-- - If status is 'ready', key MUST be NOT NULL.
-- - If status is 'processing' or 'failed', key CAN be NULL or NOT NULL (for retries).
ALTER TABLE lessons ADD CONSTRAINT chk_lessons_video CHECK (
    (video_status = 'pending' AND video_key IS NULL)
    OR
    (video_status = 'ready' AND video_key IS NOT NULL)
    OR
    (video_status IN ('processing', 'failed'))
);

COMMIT;
