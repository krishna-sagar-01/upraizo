-- ============================================================
-- Migration  : 006_create_purchases_table
-- Description: Course & E-book purchase records and Razorpay integrations
-- ============================================================

CREATE TYPE purchase_status AS ENUM ('pending', 'completed', 'failed', 'refunded');

CREATE TABLE purchases (
    id                  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID            NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    
    -- Items (Either course or ebook must be present)
    course_id           UUID            NULL REFERENCES courses (id) ON DELETE RESTRICT,
    ebook_id            UUID            NULL REFERENCES ebooks (id) ON DELETE RESTRICT,

    -- ── Payment Gateway (Razorpay) ───────────────────────────
    razorpay_order_id   VARCHAR(255)    NOT NULL,
    razorpay_payment_id VARCHAR(255)    NULL, -- NULL until payment is completed
    razorpay_signature  TEXT            NULL, -- Used for verification

    -- ── Pricing snapshot & Financials ────────────────────────
    amount_paid         NUMERIC(10,2)   NOT NULL CHECK (amount_paid >= 0),
    currency            VARCHAR(3)      NOT NULL DEFAULT 'INR',

    -- ── Extensibility & Debugging ────────────────────────────
    metadata            JSONB           NOT NULL DEFAULT '{}'::jsonb,

    -- ── Status ───────────────────────────────────────────────
    status              purchase_status NOT NULL DEFAULT 'pending',

    -- ── Access window ────────────────────────────────────────
    valid_from          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    valid_until         TIMESTAMPTZ     NULL, -- NULL means lifetime (ebooks)

    created_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    -- ── Constraints ──────────────────────────────────────────

    -- Unique Razorpay order (idempotency)
    CONSTRAINT uq_purchases_razorpay_order UNIQUE (razorpay_order_id),

    -- Valid time window (if present)
    CONSTRAINT chk_purchases_valid_window CHECK (valid_until IS NULL OR valid_until > valid_from),

    -- Either course or ebook must be purchased
    CONSTRAINT chk_purchases_item_type CHECK (
        (course_id IS NOT NULL AND ebook_id IS NULL) OR
        (course_id IS NULL AND ebook_id IS NOT NULL)
    ),

    -- Courses MUST have a valid_until date, Ebooks can have NULL (lifetime)
    CONSTRAINT chk_purchases_valid_until_req CHECK (
        (course_id IS NULL) OR (valid_until IS NOT NULL)
    ),

    -- Currency format (ISO 4217)
    CONSTRAINT chk_currency_format CHECK (currency ~ '^[A-Z]{3}$'),

    -- Strict validation for completed payments
    CONSTRAINT chk_purchases_completed_data CHECK (
        status != 'completed'
        OR (razorpay_payment_id IS NOT NULL AND razorpay_signature IS NOT NULL)
    )
);

-- ── Indexes ──────────────────────────────────────────────────

-- 1. User purchases (My Courses / Order history)
CREATE INDEX idx_purchases_user_id
    ON purchases (user_id, created_at DESC);

-- 2. Item lookups (Admin analytics)
CREATE INDEX idx_purchases_course_id ON purchases (course_id) WHERE course_id IS NOT NULL;
CREATE INDEX idx_purchases_ebook_id ON purchases (ebook_id) WHERE ebook_id IS NOT NULL;

-- 3. Access control (critical query)
CREATE UNIQUE INDEX uq_purchases_user_course_completed
    ON purchases (user_id, course_id)
    WHERE status = 'completed' AND course_id IS NOT NULL;

CREATE UNIQUE INDEX uq_purchases_user_ebook_completed
    ON purchases (user_id, ebook_id)
    WHERE status = 'completed' AND ebook_id IS NOT NULL;

-- 4. Expiry tracking (cron / reminders)
CREATE INDEX idx_purchases_expiry
    ON purchases (valid_until)
    WHERE status = 'completed' AND valid_until IS NOT NULL;

-- 5. Webhook lookup optimization
CREATE INDEX idx_purchases_order_lookup
    ON purchases (razorpay_order_id, status);

-- 6. Razorpay payment id uniqueness (only when present)
CREATE UNIQUE INDEX uq_purchases_razorpay_payment
    ON purchases (razorpay_payment_id)
    WHERE razorpay_payment_id IS NOT NULL;

-- 7. Metadata search (optional but powerful)
CREATE INDEX idx_purchases_metadata_gin
    ON purchases USING GIN (metadata);

-- ── Trigger ──────────────────────────────────────────────────
CREATE TRIGGER trg_purchases_updated_at
    BEFORE UPDATE ON purchases
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- ── Comments ─────────────────────────────────────────────────
COMMENT ON TABLE  purchases                     IS 'Course & E-book purchase records';
COMMENT ON COLUMN purchases.valid_until         IS 'Access end time (NULL for lifetime/ebooks)';