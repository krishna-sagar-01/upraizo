-- ============================================================
-- Migration  : 012_create_platform_stats_table.up.sql
-- ============================================================

-- 1. Create the platform_stats table
CREATE TABLE platform_stats (
    id                 INT         PRIMARY KEY DEFAULT 1,
    total_users        BIGINT      NOT NULL DEFAULT 0,
    active_users       BIGINT      NOT NULL DEFAULT 0,
    inactive_users     BIGINT      NOT NULL DEFAULT 0,
    suspended_users    BIGINT      NOT NULL DEFAULT 0,
    banned_users       BIGINT      NOT NULL DEFAULT 0,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT singleton_stats CHECK (id = 1)
);

-- 2. Create the Trigger Function
CREATE OR REPLACE FUNCTION fn_update_platform_stats()
RETURNS TRIGGER AS $$
BEGIN
    -- HANDLE INSERT
    IF (TG_OP = 'INSERT') THEN
        UPDATE platform_stats SET 
            total_users = total_users + 1,
            active_users = active_users + CASE WHEN NEW.status = 'active' THEN 1 ELSE 0 END,
            inactive_users = inactive_users + CASE WHEN NEW.status = 'inactive' THEN 1 ELSE 0 END,
            suspended_users = suspended_users + CASE WHEN NEW.status = 'suspended' THEN 1 ELSE 0 END,
            banned_users = banned_users + CASE WHEN NEW.status = 'banned' THEN 1 ELSE 0 END,
            updated_at = NOW()
        WHERE id = 1;

    -- HANDLE DELETE
    ELSIF (TG_OP = 'DELETE') THEN
        UPDATE platform_stats SET 
            total_users = total_users - 1,
            active_users = active_users - CASE WHEN OLD.status = 'active' THEN 1 ELSE 0 END,
            inactive_users = inactive_users - CASE WHEN OLD.status = 'inactive' THEN 1 ELSE 0 END,
            suspended_users = suspended_users - CASE WHEN OLD.status = 'suspended' THEN 1 ELSE 0 END,
            banned_users = banned_users - CASE WHEN OLD.status = 'banned' THEN 1 ELSE 0 END,
            updated_at = NOW()
        WHERE id = 1;

    -- HANDLE UPDATE (Status Transition)
    ELSIF (TG_OP = 'UPDATE') THEN
        IF (OLD.status IS DISTINCT FROM NEW.status) THEN
            UPDATE platform_stats SET 
                active_users = active_users 
                    - (CASE WHEN OLD.status = 'active' THEN 1 ELSE 0 END) 
                    + (CASE WHEN NEW.status = 'active' THEN 1 ELSE 0 END),
                inactive_users = inactive_users 
                    - (CASE WHEN OLD.status = 'inactive' THEN 1 ELSE 0 END) 
                    + (CASE WHEN NEW.status = 'inactive' THEN 1 ELSE 0 END),
                suspended_users = suspended_users 
                    - (CASE WHEN OLD.status = 'suspended' THEN 1 ELSE 0 END) 
                    + (CASE WHEN NEW.status = 'suspended' THEN 1 ELSE 0 END),
                banned_users = banned_users 
                    - (CASE WHEN OLD.status = 'banned' THEN 1 ELSE 0 END) 
                    + (CASE WHEN NEW.status = 'banned' THEN 1 ELSE 0 END),
                updated_at = NOW()
            WHERE id = 1;
        END IF;
    END IF;
    
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- 3. Attach Triggers to users table
CREATE TRIGGER trg_users_stats_sync
AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION fn_update_platform_stats();

-- 4. Initial Backfill: Calculate current counts and insert the singleton row
INSERT INTO platform_stats (
    id, total_users, active_users, inactive_users, suspended_users, banned_users, updated_at
)
SELECT 
    1,
    COUNT(*),
    COUNT(*) FILTER (WHERE status = 'active'),
    COUNT(*) FILTER (WHERE status = 'inactive'),
    COUNT(*) FILTER (WHERE status = 'suspended'),
    COUNT(*) FILTER (WHERE status = 'banned'),
    NOW()
FROM users
WHERE status != 'deleted';
