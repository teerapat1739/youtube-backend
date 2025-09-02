-- Script to verify that the unique constraint on voter_phone exists
-- Run this to check if the migration was successful

-- 1. Check if the unique constraint exists
SELECT 
    conname as constraint_name,
    contype as constraint_type,
    pg_get_constraintdef(c.oid) as constraint_definition
FROM pg_constraint c
JOIN pg_class t ON c.conrelid = t.oid
JOIN pg_namespace n ON t.relnamespace = n.oid
WHERE t.relname = 'votes' 
  AND n.nspname = 'public'
  AND conname LIKE '%phone%';

-- 2. Check if the index exists
SELECT 
    indexname,
    indexdef
FROM pg_indexes
WHERE tablename = 'votes' 
  AND indexname LIKE '%phone%';

-- 3. Check if schema_migrations table exists and has the phone migration record
SELECT version, applied_at 
FROM schema_migrations 
WHERE version = 'phone_standardization_001';

-- 4. Sample query to verify unique constraint works (this should succeed)
-- EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM votes WHERE voter_phone = '0123456789';