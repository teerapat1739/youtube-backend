-- Migration: Add performance indexes for high-load queries
-- This migration adds indexes to improve query performance for 500 RPS load testing

BEGIN;

-- Check if indexes already exist and create them if not

-- Index for fast user_id lookups (most frequent query)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes 
        WHERE tablename = 'votes' 
        AND indexname = 'idx_votes_user_id'
    ) THEN
        CREATE INDEX idx_votes_user_id ON votes(user_id);
        RAISE NOTICE 'Created index idx_votes_user_id';
    END IF;
END $$;

-- Phone index already exists from previous migrations (idx_votes_voter_phone)
-- Skipping creation to avoid duplicate indexes

-- Composite index for user status queries (user_id + vote_id + team_id)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes 
        WHERE tablename = 'votes' 
        AND indexname = 'idx_votes_user_status'
    ) THEN
        CREATE INDEX idx_votes_user_status ON votes(user_id, vote_id, team_id)
        WHERE vote_id IS NOT NULL;
        RAISE NOTICE 'Created index idx_votes_user_status';
    END IF;
END $$;

-- Index for created_at (used in sorting and time-based queries)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes 
        WHERE tablename = 'votes' 
        AND indexname = 'idx_votes_created_at'
    ) THEN
        CREATE INDEX idx_votes_created_at ON votes(created_at DESC);
        RAISE NOTICE 'Created index idx_votes_created_at';
    END IF;
END $$;

-- Index for team vote counting (used in results aggregation)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes 
        WHERE tablename = 'votes' 
        AND indexname = 'idx_votes_team_id'
    ) THEN
        CREATE INDEX idx_votes_team_id ON votes(team_id)
        WHERE team_id IS NOT NULL;
        RAISE NOTICE 'Created index idx_votes_team_id';
    END IF;
END $$;

-- Composite index for personal info lookups
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes 
        WHERE tablename = 'votes' 
        AND indexname = 'idx_votes_personal_info'
    ) THEN
        CREATE INDEX idx_votes_personal_info ON votes(user_id, voter_name, voter_email)
        WHERE voter_name IS NOT NULL AND voter_name != '';
        RAISE NOTICE 'Created index idx_votes_personal_info';
    END IF;
END $$;

-- Update table statistics for better query planning
ANALYZE votes;

-- Add comments for documentation
COMMENT ON INDEX idx_votes_user_id IS 'Primary index for user lookups - critical for GetUserStatus queries';
COMMENT ON INDEX idx_votes_user_status IS 'Composite index for user voting status queries';
COMMENT ON INDEX idx_votes_created_at IS 'Index for time-based queries and sorting';
COMMENT ON INDEX idx_votes_team_id IS 'Index for vote counting and aggregation';
COMMENT ON INDEX idx_votes_personal_info IS 'Composite index for personal information queries';

-- Log the migration completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('add_performance_indexes_001', NOW())
ON CONFLICT (version) DO NOTHING;

-- Display index sizes for monitoring
SELECT 
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) AS index_size
FROM pg_stat_user_indexes
WHERE tablename = 'votes'
ORDER BY pg_relation_size(indexrelid) DESC;

COMMIT;

-- Maintenance: Consider running these periodically
-- REINDEX INDEX CONCURRENTLY idx_votes_user_id;
-- VACUUM ANALYZE votes;