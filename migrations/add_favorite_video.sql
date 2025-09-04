-- Migration: Add favorite_video column to votes table
-- This migration adds support for storing user's favorite video field
-- from the PersonalInfoForm

BEGIN;

-- Step 1: Add favorite_video column to votes table
-- Column is nullable (optional field) with 1000 character limit
ALTER TABLE votes 
ADD COLUMN favorite_video TEXT;

-- Step 2: Add constraint to ensure favorite_video doesn't exceed 1000 characters
ALTER TABLE votes 
ADD CONSTRAINT check_favorite_video_length 
CHECK (favorite_video IS NULL OR LENGTH(favorite_video) <= 1000);

-- Step 3: Add comment for documentation
COMMENT ON COLUMN votes.favorite_video IS 'User''s favorite video (max 1000 characters, optional)';

-- Step 4: Log the migration completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('add_favorite_video_001', NOW())
ON CONFLICT (version) DO NOTHING;

COMMIT;

-- Note: This migration is safe to run on existing data as:
-- 1. The column is nullable (no default required)
-- 2. Existing records will have NULL for favorite_video
-- 3. The constraint only applies to new/updated records