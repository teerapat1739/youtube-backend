-- Script: Apply vote_id constraint fix
-- Usage: Run this script to fix the vote_id null constraint issue
-- This script can be run multiple times safely (idempotent)

\echo 'Starting vote_id constraint fix...'

BEGIN;

-- Step 1: Check current state
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE table_name = 'votes' AND constraint_name = 'votes_vote_id_key'
    ) THEN
        RAISE NOTICE 'Dropping existing unique constraint on vote_id';
        ALTER TABLE votes DROP CONSTRAINT votes_vote_id_key;
    END IF;
END $$;

-- Step 2: Make vote_id nullable if not already
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'votes' 
        AND column_name = 'vote_id' 
        AND is_nullable = 'NO'
    ) THEN
        RAISE NOTICE 'Making vote_id column nullable';
        ALTER TABLE votes ALTER COLUMN vote_id DROP NOT NULL;
    ELSE
        RAISE NOTICE 'vote_id column is already nullable';
    END IF;
END $$;

-- Step 3: Add unique constraint on non-null vote_id values
DROP INDEX IF EXISTS unique_vote_id_not_null;
CREATE UNIQUE INDEX IF NOT EXISTS unique_vote_id_not_null 
ON votes (vote_id) 
WHERE vote_id IS NOT NULL;

-- Step 4: Add check constraint to ensure vote_id when voting
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE table_name = 'votes' AND constraint_name = 'vote_id_required_when_voted'
    ) THEN
        RAISE NOTICE 'Adding constraint: vote_id required when voted';
        ALTER TABLE votes ADD CONSTRAINT vote_id_required_when_voted 
        CHECK (team_id IS NULL OR vote_id IS NOT NULL);
    ELSE
        RAISE NOTICE 'Constraint vote_id_required_when_voted already exists';
    END IF;
END $$;

-- Step 5: Update table comment
COMMENT ON COLUMN votes.vote_id IS 'Unique vote identifier, nullable to allow personal info storage without voting';

-- Step 6: Create schema_migrations table if it doesn't exist
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT NOW()
);

-- Step 7: Log the migration
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('fix_vote_id_constraint_001', NOW())
ON CONFLICT (version) DO UPDATE SET applied_at = NOW();

-- Step 8: Verify the changes
DO $$
DECLARE
    nullable_check BOOLEAN;
    constraint_exists BOOLEAN;
    index_exists BOOLEAN;
BEGIN
    -- Check if vote_id is nullable
    SELECT is_nullable = 'YES' INTO nullable_check
    FROM information_schema.columns 
    WHERE table_name = 'votes' AND column_name = 'vote_id';
    
    -- Check if check constraint exists
    SELECT EXISTS(
        SELECT 1 FROM information_schema.table_constraints 
        WHERE table_name = 'votes' AND constraint_name = 'vote_id_required_when_voted'
    ) INTO constraint_exists;
    
    -- Check if unique index exists
    SELECT EXISTS(
        SELECT 1 FROM pg_indexes 
        WHERE tablename = 'votes' AND indexname = 'unique_vote_id_not_null'
    ) INTO index_exists;
    
    IF nullable_check AND constraint_exists AND index_exists THEN
        RAISE NOTICE 'SUCCESS: All changes applied correctly';
        RAISE NOTICE '  - vote_id is now nullable: %', nullable_check;
        RAISE NOTICE '  - Check constraint exists: %', constraint_exists;
        RAISE NOTICE '  - Unique index exists: %', index_exists;
    ELSE
        RAISE EXCEPTION 'FAILED: Some changes were not applied correctly. nullable:%, constraint:%, index:%', 
                       nullable_check, constraint_exists, index_exists;
    END IF;
END $$;

COMMIT;

\echo 'Vote_id constraint fix completed successfully!'