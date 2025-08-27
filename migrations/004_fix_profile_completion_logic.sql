-- Migration 004: Fix Profile Completion Logic
-- This migration fixes the issue where profile_completed flag incorrectly requires 
-- terms_accepted AND pdpa_accepted to be true, when it should only depend on 
-- actual profile data completeness.

-- Drop the existing triggers and function (check both possible names)
DROP TRIGGER IF EXISTS update_profile_completion_trigger ON users;
DROP TRIGGER IF EXISTS trigger_update_profile_completion ON users;
DROP FUNCTION IF EXISTS update_profile_completion() CASCADE;

-- Create the corrected function that only checks profile data fields
CREATE OR REPLACE FUNCTION update_profile_completion()
RETURNS TRIGGER AS $$
BEGIN
    -- Update profile_completed based ONLY on profile data fields
    -- Not on terms_accepted or pdpa_accepted status
    NEW.profile_completed := (
        NEW.first_name IS NOT NULL AND NEW.first_name != '' AND
        NEW.last_name IS NOT NULL AND NEW.last_name != '' AND
        NEW.national_id IS NOT NULL AND NEW.national_id != '' AND
        NEW.phone IS NOT NULL AND NEW.phone != ''
    );
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Recreate the trigger
CREATE TRIGGER update_profile_completion_trigger
    BEFORE INSERT OR UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_profile_completion();

-- Update existing users to fix their profile_completed status
-- This will correct any users who have complete profile data but wrong flag
UPDATE users 
SET profile_completed = (
    first_name IS NOT NULL AND first_name != '' AND
    last_name IS NOT NULL AND last_name != '' AND
    national_id IS NOT NULL AND national_id != '' AND
    phone IS NOT NULL AND phone != ''
)
WHERE 
    -- Only update users who have complete profile data but incorrect flag
    (first_name IS NOT NULL AND first_name != '' AND
     last_name IS NOT NULL AND last_name != '' AND
     national_id IS NOT NULL AND national_id != '' AND
     phone IS NOT NULL AND phone != '') 
    AND profile_completed = FALSE;

-- Log the number of affected users
DO $$
DECLARE
    affected_count INTEGER;
BEGIN
    GET DIAGNOSTICS affected_count = ROW_COUNT;
    RAISE NOTICE 'Updated profile_completed flag for % users', affected_count;
END
$$;