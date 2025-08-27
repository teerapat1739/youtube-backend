-- Migration: Add user profile fields and PDPA compliance (FIXED VERSION)
-- This migration adds the required fields for the new authentication flow

-- Start transaction
BEGIN;

-- 1. Update users table structure
-- Add new columns for terms and PDPA acceptance
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS terms_accepted BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS terms_version VARCHAR(10),
ADD COLUMN IF NOT EXISTS pdpa_accepted BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS pdpa_version VARCHAR(10),
ADD COLUMN IF NOT EXISTS profile_completed BOOLEAN DEFAULT FALSE;

-- Update existing column names to match new frontend structure
-- Rename existing columns if they exist
DO $$
BEGIN
    -- Check if old column exists and rename it
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'users' AND column_name = 'id_card_number') THEN
        ALTER TABLE users RENAME COLUMN id_card_number TO national_id;
    END IF;
    
    -- Check if old column exists and rename it  
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'users' AND column_name = 'phone_number') THEN
        ALTER TABLE users RENAME COLUMN phone_number TO phone;
    END IF;
END
$$;

-- Add national_id column if it doesn't exist (for fresh installations)
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS national_id VARCHAR(20),
ADD COLUMN IF NOT EXISTS phone VARCHAR(20);

-- Make the new fields nullable for existing users (they'll fill them later)
DO $$
BEGIN
    -- Only alter if columns exist and are not nullable
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'users' AND column_name = 'first_name' AND is_nullable = 'NO') THEN
        ALTER TABLE users ALTER COLUMN first_name DROP NOT NULL;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'users' AND column_name = 'last_name' AND is_nullable = 'NO') THEN
        ALTER TABLE users ALTER COLUMN last_name DROP NOT NULL;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'users' AND column_name = 'national_id' AND is_nullable = 'NO') THEN
        ALTER TABLE users ALTER COLUMN national_id DROP NOT NULL;
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name = 'users' AND column_name = 'phone' AND is_nullable = 'NO') THEN
        ALTER TABLE users ALTER COLUMN phone DROP NOT NULL;
    END IF;
END
$$;

-- 2. Create terms_versions table for version management
CREATE TABLE IF NOT EXISTS terms_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    version VARCHAR(10) NOT NULL,
    type VARCHAR(10) NOT NULL CHECK (type IN ('terms', 'pdpa')),
    content TEXT NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(version, type)
);

-- 3. Create user_terms_acceptance table for audit trail
CREATE TABLE IF NOT EXISTS user_terms_acceptance (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    terms_version VARCHAR(10) NOT NULL,
    pdpa_version VARCHAR(10) NOT NULL,
    accepted_at TIMESTAMP DEFAULT NOW(),
    ip_address VARCHAR(45),
    user_agent TEXT,
    UNIQUE(user_id, terms_version, pdpa_version)
);

-- 4. Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_terms_version ON users(terms_version);
CREATE INDEX IF NOT EXISTS idx_users_pdpa_version ON users(pdpa_version);
CREATE INDEX IF NOT EXISTS idx_users_profile_completed ON users(profile_completed);
CREATE INDEX IF NOT EXISTS idx_users_national_id ON users(national_id);
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
CREATE INDEX IF NOT EXISTS idx_terms_versions_type ON terms_versions(type, active);
CREATE INDEX IF NOT EXISTS idx_user_terms_user_id ON user_terms_acceptance(user_id);
CREATE INDEX IF NOT EXISTS idx_user_terms_accepted_at ON user_terms_acceptance(accepted_at);

-- 5. Insert initial terms and PDPA versions (FIXED)
INSERT INTO terms_versions (version, type, content, active) 
SELECT '1.0', 'terms', '1. ผู้เข้าร่วมต้องเป็นผู้ติดตาม (Subscribe) ช่อง Ananped บน YouTube
2. ข้อมูลที่กรอกต้องเป็นข้อมูลจริงและถูกต้อง
3. หากพบว่าข้อมูลเป็นเท็จ ทางช่องสงวนสิทธิ์ในการตัดสิทธิ์การเข้าร่วม
4. การตัดสินของทางช่องถือเป็นที่สุด
5. ทางช่องสงวนสิทธิ์ในการเปลี่ยนแปลงเงื่อนไขโดยไม่ต้องแจ้งให้ทราบล่วงหน้า', TRUE
WHERE NOT EXISTS (SELECT 1 FROM terms_versions WHERE version = '1.0' AND type = 'terms');

INSERT INTO terms_versions (version, type, content, active) 
SELECT '1.0', 'pdpa', '1. เราจะเก็บรักษาข้อมูลส่วนบุคคลของท่านอย่างปลอดภัย
2. ข้อมูลจะใช้เพื่อการจัดกิจกรรมและติดต่อผู้ชนะเท่านั้น
3. เราจะไม่เปิดเผยข้อมูลส่วนบุคคลให้กับบุคคลที่สาม
4. ท่านมีสิทธิ์ขอดูข้อมoubl แก้ไข หรือลบข้อมูลส่วนบุคคล
5. หากมีข้อสงสัย สามารถติดต่อเราได้ตามช่องทางที่ระบุ', TRUE
WHERE NOT EXISTS (SELECT 1 FROM terms_versions WHERE version = '1.0' AND type = 'pdpa');

-- 6. Enable RLS for new tables
ALTER TABLE terms_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_terms_acceptance ENABLE ROW LEVEL SECURITY;

-- 7. Create RLS policies for new tables
-- Anyone can read active terms versions
DROP POLICY IF EXISTS "Anyone can read active terms" ON terms_versions;
CREATE POLICY "Anyone can read active terms" ON terms_versions
    FOR SELECT USING (active = TRUE);

-- Users can read their own terms acceptance records
DROP POLICY IF EXISTS "Users can read own terms acceptance" ON user_terms_acceptance;
CREATE POLICY "Users can read own terms acceptance" ON user_terms_acceptance
    FOR SELECT USING (user_id::text = auth.uid()::text);

-- Users can create terms acceptance records
DROP POLICY IF EXISTS "Users can create terms acceptance" ON user_terms_acceptance;
CREATE POLICY "Users can create terms acceptance" ON user_terms_acceptance
    FOR INSERT WITH CHECK (true);

-- 8. Create function to validate Thai National ID
CREATE OR REPLACE FUNCTION validate_thai_national_id(national_id TEXT)
RETURNS BOOLEAN AS $$
DECLARE
    id_clean TEXT;
    sum_val INTEGER := 0;
    check_digit INTEGER;
    i INTEGER;
BEGIN
    -- Remove all non-digits
    id_clean := regexp_replace(national_id, '[^0-9]', '', 'g');
    
    -- Check if exactly 13 digits
    IF length(id_clean) != 13 THEN
        RETURN FALSE;
    END IF;
    
    -- Calculate checksum
    FOR i IN 1..12 LOOP
        sum_val := sum_val + (substring(id_clean, i, 1)::INTEGER * (14 - i));
    END LOOP;
    
    -- Calculate check digit
    check_digit := (11 - (sum_val % 11)) % 10;
    
    -- Validate against last digit
    RETURN check_digit = substring(id_clean, 13, 1)::INTEGER;
END;
$$ LANGUAGE plpgsql;

-- 9. Create function to validate Thai phone number
CREATE OR REPLACE FUNCTION validate_thai_phone(phone TEXT)
RETURNS BOOLEAN AS $$
DECLARE
    phone_clean TEXT;
BEGIN
    -- Remove all non-digits
    phone_clean := regexp_replace(phone, '[^0-9]', '', 'g');
    
    -- Check if exactly 10 digits starting with 0
    IF length(phone_clean) != 10 OR substring(phone_clean, 1, 1) != '0' THEN
        RETURN FALSE;
    END IF;
    
    -- Check valid mobile prefixes (08, 09, 06)
    IF substring(phone_clean, 1, 2) NOT IN ('08', '09', '06') THEN
        RETURN FALSE;
    END IF;
    
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- 10. Create function to update profile completion status
CREATE OR REPLACE FUNCTION update_profile_completion()
RETURNS TRIGGER AS $$
BEGIN
    NEW.profile_completed := (
        NEW.first_name IS NOT NULL AND NEW.first_name != '' AND
        NEW.last_name IS NOT NULL AND NEW.last_name != '' AND
        NEW.phone IS NOT NULL AND NEW.phone != '' AND
        NEW.national_id IS NOT NULL AND NEW.national_id != '' AND
        NEW.terms_accepted = TRUE AND
        NEW.pdpa_accepted = TRUE
    );
    
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 11. Create trigger for profile completion
DROP TRIGGER IF EXISTS trigger_update_profile_completion ON users;
CREATE TRIGGER trigger_update_profile_completion
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_profile_completion();

-- 12. Add constraints for data validation (with safer approach)
DO $$
BEGIN
    -- Add constraints only if they don't exist
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_national_id_format') THEN
        ALTER TABLE users ADD CONSTRAINT check_national_id_format 
        CHECK (national_id IS NULL OR validate_thai_national_id(national_id));
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_phone_format') THEN
        ALTER TABLE users ADD CONSTRAINT check_phone_format 
        CHECK (phone IS NULL OR validate_thai_phone(phone));
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_first_name_length') THEN
        ALTER TABLE users ADD CONSTRAINT check_first_name_length 
        CHECK (first_name IS NULL OR (length(first_name) >= 2 AND length(first_name) <= 50));
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_last_name_length') THEN
        ALTER TABLE users ADD CONSTRAINT check_last_name_length 
        CHECK (last_name IS NULL OR (length(last_name) >= 2 AND length(last_name) <= 50));
    END IF;
END
$$;

-- 13. Grant permissions for new tables and functions
GRANT USAGE ON SCHEMA public TO anon, authenticated;
GRANT ALL ON TABLE terms_versions TO anon, authenticated;
GRANT ALL ON TABLE user_terms_acceptance TO anon, authenticated;
GRANT EXECUTE ON FUNCTION validate_thai_national_id(TEXT) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION validate_thai_phone(TEXT) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION update_profile_completion() TO anon, authenticated;

-- Commit transaction
COMMIT;

-- Log completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('002_user_profile_pdpa_updates', NOW()) 
ON CONFLICT (version) DO NOTHING;

-- Display migration completion message
DO $$
BEGIN
    RAISE NOTICE 'Migration 002_user_profile_pdpa_updates.sql completed successfully!';
    RAISE NOTICE 'Added: terms/PDPA fields, validation functions, and audit tables';
END
$$;
