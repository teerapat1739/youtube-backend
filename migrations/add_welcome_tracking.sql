-- Migration: Add welcome/rules acceptance tracking columns
-- This migration adds columns to track welcome acceptance status in the votes table

BEGIN;

-- Add welcome tracking columns to votes table
ALTER TABLE votes 
ADD COLUMN welcome_accepted BOOLEAN DEFAULT FALSE NOT NULL,
ADD COLUMN welcome_accepted_at TIMESTAMP NULL,
ADD COLUMN rules_version VARCHAR(50) NULL;

-- Add composite index for better performance on welcome acceptance lookups
-- This single index covers both user_id lookups and welcome_accepted filtering
CREATE INDEX IF NOT EXISTS idx_votes_user_id_welcome ON votes(user_id, welcome_accepted);

-- Add comments for documentation
COMMENT ON COLUMN votes.welcome_accepted IS 'Whether the user has accepted the welcome/rules';
COMMENT ON COLUMN votes.welcome_accepted_at IS 'Timestamp when welcome/rules were accepted';
COMMENT ON COLUMN votes.rules_version IS 'Version of the rules that were accepted';

-- Log the migration completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('add_welcome_tracking_001', NOW())
ON CONFLICT (version) DO NOTHING;

COMMIT;