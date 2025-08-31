-- Migration: Standardize phone numbers and add unique constraint
-- This migration:
-- 1. Normalizes existing phone numbers (removes hyphens, spaces, etc.)
-- 2. Adds a unique constraint on voter_phone column
-- 3. Handles potential duplicates by keeping the earliest record

BEGIN;

-- Step 1: Create a temporary function to normalize phone numbers
CREATE OR REPLACE FUNCTION normalize_phone(phone_input TEXT) 
RETURNS TEXT AS $$
BEGIN
    -- Remove all non-digit characters
    phone_input := regexp_replace(phone_input, '[^0-9]', '', 'g');
    
    -- Handle international format (+66)
    IF phone_input ~ '^66' AND length(phone_input) >= 10 THEN
        phone_input := '0' || substring(phone_input, 3);
    END IF;
    
    -- Validate Thai phone number format (10 digits starting with 0)
    IF phone_input ~ '^0[0-9]{9}$' THEN
        RETURN phone_input;
    END IF;
    
    -- Return original if invalid (will be handled by application validation)
    RETURN phone_input;
END;
$$ LANGUAGE plpgsql;

-- Step 2: Update all existing phone numbers to normalized format
UPDATE votes 
SET voter_phone = normalize_phone(voter_phone)
WHERE voter_phone IS NOT NULL;

-- Step 3: Handle potential duplicates by keeping the earliest vote for each phone number
-- First, identify duplicate phone numbers
WITH duplicate_phones AS (
    SELECT voter_phone, COUNT(*) as count_records, MIN(created_at) as earliest_vote
    FROM votes 
    WHERE voter_phone IS NOT NULL AND voter_phone != ''
    GROUP BY voter_phone 
    HAVING COUNT(*) > 1
),
-- Get the IDs of records to delete (all duplicates except the earliest)
records_to_delete AS (
    SELECT v.id
    FROM votes v
    INNER JOIN duplicate_phones dp ON v.voter_phone = dp.voter_phone
    WHERE v.created_at > dp.earliest_vote
)
-- Delete duplicate records (keeping the earliest vote for each phone number)
DELETE FROM votes 
WHERE id IN (SELECT id FROM records_to_delete);

-- Step 4: Add unique constraint on voter_phone
ALTER TABLE votes 
ADD CONSTRAINT unique_voter_phone 
UNIQUE (voter_phone);

-- Step 5: Add index for better performance on phone lookups
CREATE INDEX IF NOT EXISTS idx_votes_voter_phone ON votes(voter_phone);

-- Step 6: Drop the temporary function
DROP FUNCTION normalize_phone(TEXT);

-- Step 7: Log the migration completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('phone_standardization_001', NOW())
ON CONFLICT (version) DO NOTHING;

COMMIT;

-- Create schema_migrations table if it doesn't exist
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT NOW()
);