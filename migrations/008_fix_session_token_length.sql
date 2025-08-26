-- Fix session_token column length limitation
-- Migration: 008_fix_session_token_length.sql
-- 
-- Problem: JWT tokens can exceed 255 characters but session_token is VARCHAR(255)
-- Solution: Change session_token to TEXT to handle longer tokens
--
-- JWT tokens with user_id, google_id, email, exp, iat, iss claims + signature
-- can easily be 300-500+ characters depending on email length and Google ID

-- First, let's check current max length of existing session tokens
-- (This is informational - will show in logs during migration)
DO $$
DECLARE
    max_length INTEGER;
    count_over_255 INTEGER;
BEGIN
    SELECT MAX(LENGTH(session_token)), COUNT(*)
    FROM user_sessions
    WHERE LENGTH(session_token) > 255
    INTO max_length, count_over_255;
    
    RAISE NOTICE 'Current max session_token length: %, Count over 255 chars: %', max_length, count_over_255;
END $$;

-- Change session_token from VARCHAR(255) to TEXT
-- This allows unlimited length tokens
ALTER TABLE user_sessions 
ALTER COLUMN session_token TYPE TEXT;

-- Update the unique constraint to work with TEXT
-- First drop the existing unique constraint on session_token
ALTER TABLE user_sessions 
DROP CONSTRAINT IF EXISTS user_sessions_session_token_key;

-- Recreate the unique constraint (TEXT fields can have unique constraints)
ALTER TABLE user_sessions 
ADD CONSTRAINT user_sessions_session_token_key UNIQUE (session_token);

-- Update the index to handle TEXT efficiently
-- Drop existing index
DROP INDEX IF EXISTS idx_user_sessions_token;

-- Create new index optimized for TEXT fields
-- Using hash index for exact matches (session token lookups are always exact)
CREATE INDEX idx_user_sessions_token_hash ON user_sessions USING hash (session_token);

-- Also create a btree index for range queries and general performance
CREATE INDEX idx_user_sessions_token_btree ON user_sessions USING btree (session_token);

-- Add comments for documentation
COMMENT ON COLUMN user_sessions.session_token IS 'JWT session token (changed from VARCHAR(255) to TEXT to handle longer tokens)';

-- Log completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('008_fix_session_token_length', NOW()) 
ON CONFLICT (version) DO NOTHING;

-- Verify the change
DO $$
DECLARE
    col_type TEXT;
BEGIN
    SELECT data_type INTO col_type
    FROM information_schema.columns
    WHERE table_name = 'user_sessions' AND column_name = 'session_token';
    
    RAISE NOTICE 'session_token column is now: %', col_type;
    
    IF col_type != 'text' THEN
        RAISE EXCEPTION 'Migration failed: session_token is still %, expected text', col_type;
    END IF;
    
    RAISE NOTICE '✅ Migration completed successfully - session_token is now TEXT';
END $$;