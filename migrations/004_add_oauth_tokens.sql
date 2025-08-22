-- Add OAuth token fields to users table for YouTube API access
-- Migration: 004_add_oauth_tokens.sql

-- Add OAuth token columns to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_access_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_refresh_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_token_expiry TIMESTAMP;
ALTER TABLE users ADD COLUMN IF NOT EXISTS youtube_channel_id TEXT;

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_google_access_token ON users(google_access_token);
CREATE INDEX IF NOT EXISTS idx_users_youtube_channel_id ON users(youtube_channel_id);

-- Add comments for documentation
COMMENT ON COLUMN users.google_access_token IS 'Google OAuth access token for YouTube API access';
COMMENT ON COLUMN users.google_refresh_token IS 'Google OAuth refresh token for token renewal';
COMMENT ON COLUMN users.google_token_expiry IS 'Expiry time for the Google OAuth access token';
COMMENT ON COLUMN users.youtube_channel_id IS 'Users YouTube channel ID for subscription verification';

-- Log completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('004_add_oauth_tokens', NOW()) 
ON CONFLICT (version) DO NOTHING;