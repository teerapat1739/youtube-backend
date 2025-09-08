-- Script to apply the unique phone constraint fix
-- Run this script against your database to fix the constraint issue
-- Usage: psql -h [host] -p [port] -U [user] -d [database] -f apply_phone_constraint_fix.sql

\echo 'Applying unique phone constraint fix...'

-- Execute the migration
\i ../migrations/fix_unique_phone_constraint.sql

-- Verify the fix
\echo 'Verifying the fix...'

-- Check constraint exists and allows multiple NULLs
SELECT 
    'Unique constraint exists: ' || 
    CASE WHEN COUNT(*) > 0 THEN 'YES' ELSE 'NO' END as constraint_check
FROM pg_constraint c
JOIN pg_class t ON c.conrelid = t.oid
JOIN pg_namespace n ON t.relnamespace = n.oid
WHERE t.relname = 'votes' 
  AND n.nspname = 'public'
  AND c.conname = 'unique_voter_phone';

-- Check for empty string phone numbers (should be 0 after migration)
SELECT 
    'Empty phone strings found: ' || COUNT(*) as empty_phone_check
FROM votes 
WHERE voter_phone = '';

-- Check for NULL phone numbers (should be > 0 after migration)  
SELECT 
    'NULL phone records found: ' || COUNT(*) as null_phone_check
FROM votes 
WHERE voter_phone IS NULL;

-- Test that we can insert multiple NULL phone records (this should work)
\echo 'Testing multiple NULL phone insertions...'
BEGIN;
-- These should all succeed
INSERT INTO votes (user_id, voter_name, voter_email, voter_phone, welcome_accepted, welcome_accepted_at, rules_version) 
VALUES 
    ('test_user_1', '', '', NULL, true, NOW(), 'v1.0'),
    ('test_user_2', '', '', NULL, true, NOW(), 'v1.0'),
    ('test_user_3', '', '', NULL, true, NOW(), 'v1.0');

\echo 'Multiple NULL phone insertions successful!';

-- Clean up test data
DELETE FROM votes WHERE user_id IN ('test_user_1', 'test_user_2', 'test_user_3');
ROLLBACK;

\echo 'Phone constraint fix applied and verified successfully!';