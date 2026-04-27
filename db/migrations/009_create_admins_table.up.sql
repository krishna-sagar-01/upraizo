-- ============================================================
-- Migration  : 011_create_admins_table
-- ============================================================

CREATE TABLE admins (
    id                  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    name                TEXT            NOT NULL,
    email               CITEXT          NOT NULL,
    phone               TEXT            NOT NULL,
    password_hash       TEXT            NOT NULL,
    secret_key_hash     TEXT            NOT NULL,
    is_active           BOOLEAN         NOT NULL DEFAULT TRUE,
    
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    -- ── Constraints ──────────────────────────────────────────
    CONSTRAINT uq_admins_email UNIQUE (email),
    CONSTRAINT uq_admins_phone UNIQUE (phone),
    
    -- Basic validation for email and phone
    CONSTRAINT chk_admins_email CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
    CONSTRAINT chk_admins_phone CHECK (phone ~ '^\+?[1-9]\d{1,14}$')
);

-- ── Indexes ──────────────────────────────────────────────────

-- Email lookup (most common)
CREATE INDEX idx_admins_email ON admins (email);

-- Phone lookup
CREATE INDEX idx_admins_phone ON admins (phone);

-- Active admins index
CREATE INDEX idx_admins_active ON admins (is_active) WHERE is_active = TRUE;

-- ── Trigger ──────────────────────────────────────────────────
CREATE TRIGGER trg_admins_updated_at
    BEFORE UPDATE ON admins
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- ── Comments ─────────────────────────────────────────────────
COMMENT ON TABLE  admins                  IS 'Restricted administrative users';
COMMENT ON COLUMN admins.password_hash    IS 'Bcrypt hash of the administrative password';
COMMENT ON COLUMN admins.secret_key_hash  IS 'Bcrypt hash of the secondary secret key required for login';
