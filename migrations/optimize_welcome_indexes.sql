-- Migration: Optimize welcome tracking indexes
-- Remove redundant single-column index on welcome_accepted

BEGIN;

-- Drop redundant index (composite index covers all use cases)
DROP INDEX IF EXISTS idx_votes_welcome_accepted;

-- Log the optimization
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('optimize_welcome_indexes_001', NOW())
ON CONFLICT (version) DO NOTHING;

COMMIT;