-- Migration: Fix profile completion trigger to remove national_id reference
-- Description: Update the update_profile_completion() function to work without national_id
-- Date: 2025-08-27

-- Drop and recreate the trigger function without national_id reference
CREATE OR REPLACE FUNCTION update_profile_completion()
RETURNS TRIGGER AS $$
BEGIN
    -- Update profile_completed based ONLY on profile data fields
    -- Not on terms_accepted or pdpa_accepted status
    NEW.profile_completed := (
        NEW.first_name IS NOT NULL AND NEW.first_name != '' AND
        NEW.last_name IS NOT NULL AND NEW.last_name != '' AND
        NEW.phone IS NOT NULL AND NEW.phone != ''
    );

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Add comment explaining the change
COMMENT ON FUNCTION update_profile_completion() IS 'Updates profile completion status based on first_name, last_name, and phone only (national_id removed)';