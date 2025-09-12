-- Migration: Add performance indexes for high-load queries
-- This migration adds indexes to improve query performance for 500 RPS load testing

BEGIN;

-- Index for fast user_id lookups (most frequent query)
CREATE INDEX IF NOT EXISTS idx_votes_user_id ON votes(user_id);

-- Phone index already exists from previous migrations (idx_votes_voter_phone)
-- Skipping creation to avoid duplicate indexes

-- Composite index for user status queries (user_id + vote_id + team_id)
CREATE INDEX IF NOT EXISTS idx_votes_user_status ON votes(user_id, vote_id, team_id)
WHERE vote_id IS NOT NULL;

-- Index for created_at (used in sorting and time-based queries)
CREATE INDEX IF NOT EXISTS idx_votes_created_at ON votes(created_at DESC);

-- Index for team vote counting (used in results aggregation)
CREATE INDEX IF NOT EXISTS idx_votes_team_id ON votes(team_id)
WHERE team_id IS NOT NULL;

-- Composite index for personal info lookups
CREATE INDEX IF NOT EXISTS idx_votes_personal_info ON votes(user_id, voter_name, voter_email)
WHERE voter_name IS NOT NULL AND voter_name != '';

-- Update table statistics for better query planning
ANALYZE votes;

-- Add comments for documentation
COMMENT ON INDEX idx_votes_user_id IS 'Primary index for user lookups - critical for GetUserStatus queries';
COMMENT ON INDEX idx_votes_user_status IS 'Composite index for user voting status queries';
COMMENT ON INDEX idx_votes_created_at IS 'Index for time-based queries and sorting';
COMMENT ON INDEX idx_votes_team_id IS 'Index for vote counting and aggregation';
COMMENT ON INDEX idx_votes_personal_info IS 'Composite index for personal information queries';

COMMIT;

-- Maintenance: Consider running these periodically
-- REINDEX INDEX CONCURRENTLY idx_votes_user_id;
-- VACUUM ANALYZE votes;