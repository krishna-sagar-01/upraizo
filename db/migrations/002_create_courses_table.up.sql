-- ============================================================
-- Migration  : 002_create_courses_table
-- ============================================================

CREATE TABLE courses (
    id                  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    title               TEXT            NOT NULL,
    slug                CITEXT          NOT NULL,
    description         TEXT            NULL,
    thumbnail_url       TEXT            NULL,
    category            TEXT            NOT NULL,

    -- ── Pricing ──────────────────────────────────────────────
    price               NUMERIC(10,2)   NOT NULL CHECK (price >= 0),
    original_price      NUMERIC(10,2)   NULL CHECK (original_price >= 0),
    discount_label      TEXT            NULL,
    discount_expires_at TIMESTAMPTZ     NULL,

    -- ── Access ───────────────────────────────────────────────
    validity_days       INTEGER         NOT NULL CHECK (validity_days > 0),

    -- ── State ────────────────────────────────────────────────
    is_published        BOOLEAN         NOT NULL DEFAULT FALSE,
    razorpay_item_id    TEXT            NULL,

    created_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    -- ── Constraints ──────────────────────────────────────────

    CONSTRAINT uq_courses_slug UNIQUE (slug),

    -- Discount logic
    CONSTRAINT chk_courses_discount CHECK (
        original_price IS NULL OR original_price > price
    ),

    CONSTRAINT chk_courses_discount_fields CHECK (
        (original_price IS NULL AND discount_label IS NULL AND discount_expires_at IS NULL)
        OR
        (original_price IS NOT NULL)
    ),

    -- URL validation
    CONSTRAINT chk_thumbnail_url CHECK (
        thumbnail_url IS NULL OR thumbnail_url ~ '^https?://'
    )
);

-- ── Indexes ──────────────────────────────────────────────────

-- Published courses (main listing)
CREATE INDEX idx_courses_published
    ON courses (created_at DESC)
    WHERE is_published = TRUE;

-- Category filter
CREATE INDEX idx_courses_category
    ON courses (category)
    WHERE is_published = TRUE;

-- Slug lookup (API)
CREATE INDEX idx_courses_slug
    ON courses (slug);

-- Discount expiry (cron jobs)
CREATE INDEX idx_courses_discount_expiry
    ON courses (discount_expires_at)
    WHERE discount_expires_at IS NOT NULL;

-- Optional: search optimization (future)
CREATE INDEX idx_courses_title_search
    ON courses USING GIN (to_tsvector('english', title));

-- ── Trigger ──────────────────────────────────────────────────
CREATE TRIGGER trg_courses_updated_at
    BEFORE UPDATE ON courses
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- ── Comments ─────────────────────────────────────────────────
COMMENT ON TABLE  courses                     IS 'Course catalog';
COMMENT ON COLUMN courses.slug                IS 'Case-insensitive URL-friendly identifier';
COMMENT ON COLUMN courses.category            IS 'Course category (consider enum/table in future)';
COMMENT ON COLUMN courses.price               IS 'Actual selling price (what the user pays)';
COMMENT ON COLUMN courses.original_price      IS 'MRP / crossed-out price. NULL means no discount';
COMMENT ON COLUMN courses.discount_label      IS 'Badge text (e.g. LAUNCH OFFER, 50% OFF)';
COMMENT ON COLUMN courses.discount_expires_at IS 'Discount end time';
COMMENT ON COLUMN courses.validity_days       IS 'Access duration in days after purchase';
COMMENT ON COLUMN courses.is_published        IS 'FALSE = draft mode';