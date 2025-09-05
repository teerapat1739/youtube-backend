-- Migration: Fix unique phone constraint to handle NULL values properly
-- This migration:
-- 1. Updates empty string phone values to NULL
-- 2. Recreates the unique constraint to properly handle NULL values  
-- 3. Ensures multiple users can accept welcome without phone conflicts

BEGIN;

-- Step 1: Update all empty string voter_phone values to NULL
-- This is safe because empty strings should never be valid phone numbers
UPDATE votes 
SET voter_phone = NULL 
WHERE voter_phone = '' OR voter_phone IS NULL;

-- Step 2: Drop the existing unique constraint (if it exists)
-- The constraint prevents multiple empty strings but should allow multiple NULLs
DO $$ 
BEGIN
    -- Check if constraint exists before dropping
    IF EXISTS (
        SELECT 1 FROM pg_constraint c
        JOIN pg_class t ON c.conrelid = t.oid
        WHERE t.relname = 'votes' 
        AND c.conname = 'unique_voter_phone'
    ) THEN
        ALTER TABLE votes DROP CONSTRAINT unique_voter_phone;
    END IF;
END $$;

-- Step 3: Recreate the unique constraint 
-- In PostgreSQL, NULL values are not considered equal for unique constraints
-- So this will allow multiple NULL values while preventing duplicate phone numbers
ALTER TABLE votes 
ADD CONSTRAINT unique_voter_phone 
UNIQUE (voter_phone);

-- Step 4: Add a partial index for better performance (optional but recommended)
-- This creates an index only on non-NULL phone numbers
DROP INDEX IF EXISTS idx_votes_voter_phone;
CREATE INDEX idx_votes_voter_phone ON votes(voter_phone) 
WHERE voter_phone IS NOT NULL;

-- Step 5: Add check constraint to ensure phone numbers are either NULL or valid format
-- This prevents empty strings from being inserted in the future
DO $$
BEGIN
    -- Drop check constraint if it exists
    IF EXISTS (
        SELECT 1 FROM pg_constraint c
        JOIN pg_class t ON c.conrelid = t.oid
        WHERE t.relname = 'votes' 
        AND c.conname = 'check_voter_phone_not_empty'
    ) THEN
        ALTER TABLE votes DROP CONSTRAINT check_voter_phone_not_empty;
    END IF;
END $$;

ALTER TABLE votes 
ADD CONSTRAINT check_voter_phone_not_empty 
CHECK (voter_phone IS NULL OR length(trim(voter_phone)) > 0);

-- Step 6: Log the migration completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('fix_unique_phone_constraint_001', NOW())
ON CONFLICT (version) DO NOTHING;

COMMIT;