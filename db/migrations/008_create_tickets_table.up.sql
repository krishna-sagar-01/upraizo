-- ============================================================
-- Migration  : 008_create_tickets_table
-- ============================================================


CREATE TYPE ticket_status AS ENUM ('open', 'in_progress', 'resolved', 'closed');
CREATE TYPE ticket_priority AS ENUM ('low', 'medium', 'high', 'urgent');

CREATE TABLE tickets (
    id          UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID            NOT NULL REFERENCES users (id) ON DELETE RESTRICT,

    subject     TEXT            NOT NULL,
    category    TEXT            NOT NULL DEFAULT 'general',
    
    priority    ticket_priority NOT NULL DEFAULT 'medium',
    status      ticket_status   NOT NULL DEFAULT 'open',

    metadata    JSONB           NOT NULL DEFAULT '{}'::jsonb,

    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    -- Basic validation
    CONSTRAINT chk_tickets_subject CHECK (char_length(subject) >= 3)
);

-- ── Indexes ──────────────────────────────────────────────────

-- User tickets (history page)
CREATE INDEX idx_tickets_user_id
    ON tickets (user_id, created_at DESC);

-- Admin queue (sorted by priority + FIFO)
CREATE INDEX idx_tickets_admin_queue
    ON tickets (status, priority, created_at ASC)
    WHERE status IN ('open', 'in_progress');

-- Optional: category filtering (admin dashboard)
CREATE INDEX idx_tickets_category
    ON tickets (category);

-- Optional: metadata search (future debugging filters)
CREATE INDEX idx_tickets_metadata_gin
    ON tickets USING GIN (metadata);

-- ── Trigger ──────────────────────────────────────────────────
CREATE TRIGGER trg_tickets_updated_at
    BEFORE UPDATE ON tickets
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- ── Comments ─────────────────────────────────────────────────
COMMENT ON TABLE  tickets          IS 'User support tickets';
COMMENT ON COLUMN tickets.priority IS 'Helps admins sort tickets. urgent = critical issues like payment failure';
COMMENT ON COLUMN tickets.metadata IS 'Client side details like Browser, OS version for easier debugging';


