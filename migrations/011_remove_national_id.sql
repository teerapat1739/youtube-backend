-- Migration: Remove national_id column
-- Description: Remove national_id from users table as it's no longer needed
-- Date: 2025-08-27

-- Drop the national_id column
ALTER TABLE users DROP COLUMN IF EXISTS national_id;

-- Update profile_completed logic to check only first_name, last_name, and phone
-- This is already handled in the application logic, but we can update existing records
UPDATE users 
SET profile_completed = (
    first_name IS NOT NULL AND first_name != '' AND 
    last_name IS NOT NULL AND last_name != '' AND 
    phone IS NOT NULL AND phone != ''
)
WHERE profile_completed = FALSE;

-- Add comment explaining the change
COMMENT ON TABLE users IS 'User profiles - national_id removed, only first_name, last_name, phone required for profile completion';