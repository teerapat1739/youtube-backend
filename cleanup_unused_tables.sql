-- Cleanup script to drop unused tables from voting system database
-- This script removes tables that are no longer needed for the voting system
-- Keeps only: teams, votes, vote_summary, and schema_migrations

-- Drop tables in order to avoid foreign key constraint violations
-- Tables with foreign keys must be dropped before their referenced tables

BEGIN;

-- Drop tables that reference users table first
DROP TABLE IF EXISTS user_sessions CASCADE;
DROP TABLE IF EXISTS user_terms_acceptance CASCADE;

-- Drop remaining unused tables
DROP TABLE IF EXISTS profiles CASCADE;
DROP TABLE IF EXISTS activities CASCADE;
DROP TABLE IF EXISTS terms_versions CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- Verify remaining tables (should only show teams, votes, vote_summary, schema_migrations)
SELECT 'Remaining tables:' as info;
SELECT schemaname, tablename, tableowner 
FROM pg_tables 
WHERE schemaname = 'public' 
ORDER BY tablename;

SELECT 'Remaining materialized views:' as info;
SELECT schemaname, matviewname, matviewowner 
FROM pg_matviews 
WHERE schemaname = 'public' 
ORDER BY matviewname;

COMMIT;

-- Success message
SELECT 'Database cleanup completed successfully. Unused tables have been dropped.' as result;