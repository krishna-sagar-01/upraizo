-- ============================================================
-- Migration  : 012_create_ebooks_table
-- Description: Create ebooks table for selling digital books
-- ============================================================

CREATE TABLE ebooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    thumbnail_url TEXT,
    file_url TEXT NOT NULL,
    price NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
    original_price NUMERIC(10, 2),
    discount_label VARCHAR(50),
    discount_expires_at TIMESTAMPTZ,
    is_published BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT chk_ebooks_price_positive CHECK (price >= 0),
    CONSTRAINT chk_ebooks_discount CHECK (
        (original_price IS NULL AND discount_label IS NULL AND discount_expires_at IS NULL) OR
        (original_price IS NOT NULL AND original_price > price)
    )
);

-- Indexes
CREATE INDEX idx_ebooks_slug ON ebooks(slug);
CREATE INDEX idx_ebooks_published ON ebooks(is_published) WHERE is_published = TRUE;
CREATE INDEX idx_ebooks_created_at ON ebooks(created_at DESC);

-- Trigger for updated_at
CREATE TRIGGER set_ebooks_updated_at
BEFORE UPDATE ON ebooks
FOR EACH ROW
EXECUTE FUNCTION fn_set_updated_at();
