-- ============================================================
-- Migration  : 001_create_users_table.up.sql
-- ============================================================

-- ── Enum Types ───────────────────────────────────────────────
CREATE TYPE user_status AS ENUM ('active', 'inactive', 'banned', 'suspended', 'deleted');
CREATE TYPE auth_provider AS ENUM ('email', 'google', 'github');

-- ── Table ────────────────────────────────────────────────────
CREATE TABLE users (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT        NOT NULL,
    avatar_url        TEXT        NULL,

    email             CITEXT      NOT NULL,
    password_hash     TEXT        NULL,

    auth_provider     auth_provider NOT NULL DEFAULT 'email',
    auth_provider_id  TEXT        NULL,

    status            user_status NOT NULL DEFAULT 'inactive', -- inactive until email is verified
    status_reason     TEXT        NULL,
    is_verified       BOOLEAN     NOT NULL DEFAULT FALSE,
    verified_at       TIMESTAMPTZ NULL,                        -- NULL until verification is completed

    preferences       JSONB       NOT NULL DEFAULT '{
        "notifications": {
            "email": true,
            "website": true
        },
        "theme": "system"
    }',

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- ── Constraints ──────────────────────────────────────────

    CONSTRAINT uq_users_email UNIQUE (email),

    CONSTRAINT chk_users_auth CHECK (
        (auth_provider = 'email' AND password_hash IS NOT NULL AND auth_provider_id IS NULL)
        OR
        (auth_provider != 'email' AND auth_provider_id IS NOT NULL AND password_hash IS NULL)
    ),

    CONSTRAINT chk_users_verified CHECK (
        (is_verified = FALSE AND verified_at IS NULL)
        OR
        (is_verified = TRUE AND verified_at IS NOT NULL)
    ),

    CONSTRAINT chk_avatar_url CHECK (
        avatar_url IS NULL OR avatar_url ~ '^https?://'
    )
);

-- ── Indexes ──────────────────────────────────────────────────

-- Status filtering
CREATE INDEX idx_users_status
    ON users (status);

-- Pagination
CREATE INDEX idx_users_created_at
    ON users (created_at DESC);

-- Email login optimization
CREATE INDEX idx_users_email_login
    ON users (email)
    WHERE auth_provider = 'email';

-- OAuth users lookup (partial unique index)
CREATE UNIQUE INDEX uq_users_oauth
    ON users (auth_provider, auth_provider_id)
    WHERE auth_provider != 'email';

-- Unverified users cleanup
CREATE INDEX idx_users_unverified
    ON users (created_at)
    WHERE is_verified = FALSE;

-- ── Trigger ──────────────────────────────────────────────────
CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- ── Comments ─────────────────────────────────────────────────
COMMENT ON TABLE  users                     IS 'Platform users — supports email/password and OAuth authentication';
COMMENT ON COLUMN users.password_hash       IS 'Bcrypt hash. NULL when user uses OAuth only';
COMMENT ON COLUMN users.auth_provider       IS 'Authentication provider (email, google, github)';
COMMENT ON COLUMN users.auth_provider_id    IS 'OAuth subject ID. NULL for email/password users';
COMMENT ON COLUMN users.status              IS 'inactive = pending verification, active = normal user, banned/suspended = admin action';
COMMENT ON COLUMN users.is_verified         IS 'TRUE when email is verified or OAuth registration is completed';
COMMENT ON COLUMN users.verified_at         IS 'Timestamp when verification occurred';
COMMENT ON COLUMN users.preferences         IS 'User preferences: notifications (email, website) and theme';