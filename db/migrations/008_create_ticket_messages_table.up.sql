-- ============================================================
-- Migration  : 009_create_ticket_messages_table
-- ============================================================

CREATE TABLE ticket_messages (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID        NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,

    user_id     UUID        NULL REFERENCES users (id) ON DELETE CASCADE,
    admin_id    UUID        NULL, -- Ideally REFERENCES admins(id)

    message     TEXT        NOT NULL,
    attachments JSONB       NOT NULL DEFAULT '[]'::jsonb,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure exactly ONE sender
    CONSTRAINT chk_ticket_messages_sender CHECK (
        (user_id IS NOT NULL AND admin_id IS NULL)
        OR
        (admin_id IS NOT NULL AND user_id IS NULL)
    ),

    -- Prevent empty messages
    CONSTRAINT chk_ticket_messages_content CHECK (
        char_length(message) > 0 OR jsonb_array_length(attachments) > 0
    )
);

-- ── Indexes ──────────────────────────────────────────────────

-- Fetch messages (chat-like UI)
CREATE INDEX idx_ticket_messages_ticket_id
    ON ticket_messages (ticket_id, created_at ASC);

-- Fast lookup of user's messages (optional analytics)
CREATE INDEX idx_ticket_messages_user
    ON ticket_messages (user_id)
    WHERE user_id IS NOT NULL;

-- ── Comments ─────────────────────────────────────────────────
COMMENT ON TABLE  ticket_messages             IS 'Messages inside a support ticket';
COMMENT ON COLUMN ticket_messages.attachments IS 'Array of file URLs (e.g., Cloudflare R2 keys for screenshots)';


