-- Migration: 014_relax_lesson_video_constraint (Down)
-- Description: Restore strict video consistency constraint.

BEGIN;

ALTER TABLE lessons DROP CONSTRAINT IF EXISTS chk_lessons_video;

ALTER TABLE lessons ADD CONSTRAINT chk_lessons_video CHECK (
    (video_key IS NULL AND video_status = 'pending')
    OR
    (video_key IS NOT NULL AND video_status IN ('processing', 'ready', 'failed'))
);

COMMIT;
