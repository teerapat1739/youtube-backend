-- Migration: Fix vote_id constraint to support personal info without votes
-- This migration makes vote_id nullable to allow personal information 
-- to be stored independently from voting actions

BEGIN;

-- Step 1: Drop the unique constraint on vote_id temporarily
ALTER TABLE votes DROP CONSTRAINT IF EXISTS votes_vote_id_key;

-- Step 2: Make vote_id nullable to allow personal info without votes
ALTER TABLE votes ALTER COLUMN vote_id DROP NOT NULL;

-- Step 3: Add a unique constraint on vote_id where it's not null
-- This ensures vote_id uniqueness when present but allows NULL values
CREATE UNIQUE INDEX unique_vote_id_not_null 
ON votes (vote_id) 
WHERE vote_id IS NOT NULL;

-- Step 4: Add check constraint to ensure vote_id is present when team_id is set
-- This maintains data integrity: if someone votes, they must have a vote_id
ALTER TABLE votes ADD CONSTRAINT vote_id_required_when_voted 
CHECK (team_id IS NULL OR vote_id IS NOT NULL);

-- Step 5: Add helpful comment
COMMENT ON COLUMN votes.vote_id IS 'Unique vote identifier, nullable to allow personal info storage without voting';

-- Step 6: Log the migration completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('fix_vote_id_constraint_001', NOW())
ON CONFLICT (version) DO NOTHING;

COMMIT;