-- Complete Database Migration for Neon
-- This script contains the complete schema based on all existing migrations
-- Run this on your new Neon database instance

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create schema_migrations table first
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- MAIN TABLES
-- ============================================================================

-- Users table (based on all migrations combined)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    google_id VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    phone VARCHAR(20),
    youtube_subscribed BOOLEAN DEFAULT FALSE,
    subscription_verified_at TIMESTAMP,
    
    -- OAuth tokens (from migration 004)
    google_access_token TEXT,
    google_refresh_token TEXT,
    google_token_expiry TIMESTAMP,
    youtube_channel_id TEXT,
    
    -- Profile completion and terms acceptance (from migration 002)
    terms_accepted BOOLEAN DEFAULT FALSE,
    terms_version VARCHAR(10),
    pdpa_accepted BOOLEAN DEFAULT FALSE,
    pdpa_version VARCHAR(10),
    profile_completed BOOLEAN DEFAULT FALSE,
    
    -- Activity rules acceptance (from migration 010)
    activity_rules_accepted BOOLEAN DEFAULT FALSE,
    activity_rules_accepted_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Activities table
CREATE TABLE IF NOT EXISTS activities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    max_participants INTEGER,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Teams table
CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    activity_id UUID REFERENCES activities(id) ON DELETE CASCADE,
    name VARCHAR(10) NOT NULL, -- A, B, C, D, E, F, G, H
    display_name VARCHAR(100) NOT NULL,
    image_url VARCHAR(500),
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Votes table
CREATE TABLE IF NOT EXISTS votes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
    activity_id UUID REFERENCES activities(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, activity_id) -- One vote per user per activity
);

-- User sessions table (updated with longer token length from migration 008)
CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    session_token VARCHAR(500) UNIQUE NOT NULL, -- Increased from 255 to 500
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Terms versions table (from migration 002)
CREATE TABLE IF NOT EXISTS terms_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    version VARCHAR(10) NOT NULL,
    type VARCHAR(10) NOT NULL CHECK (type IN ('terms', 'pdpa')),
    content TEXT NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(version, type)
);

-- User terms acceptance table (from migration 002)
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

-- ============================================================================
-- INDEXES FOR PERFORMANCE
-- ============================================================================

-- Users indexes
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_google_access_token ON users(google_access_token);
CREATE INDEX IF NOT EXISTS idx_users_youtube_channel_id ON users(youtube_channel_id);
CREATE INDEX IF NOT EXISTS idx_users_terms_version ON users(terms_version);
CREATE INDEX IF NOT EXISTS idx_users_pdpa_version ON users(pdpa_version);
CREATE INDEX IF NOT EXISTS idx_users_profile_completed ON users(profile_completed);
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);

-- Vote indexes
CREATE INDEX IF NOT EXISTS idx_votes_activity_team ON votes(activity_id, team_id);
CREATE INDEX IF NOT EXISTS idx_votes_user_activity ON votes(user_id, activity_id);
CREATE INDEX IF NOT EXISTS idx_votes_created_at ON votes(created_at);

-- Team indexes
CREATE INDEX IF NOT EXISTS idx_teams_activity ON teams(activity_id);

-- Session indexes
CREATE INDEX IF NOT EXISTS idx_user_sessions_user ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_token ON user_sessions(session_token);

-- Terms indexes
CREATE INDEX IF NOT EXISTS idx_terms_versions_type ON terms_versions(type, active);
CREATE INDEX IF NOT EXISTS idx_user_terms_user_id ON user_terms_acceptance(user_id);
CREATE INDEX IF NOT EXISTS idx_user_terms_accepted_at ON user_terms_acceptance(accepted_at);

-- ============================================================================
-- FUNCTIONS
-- ============================================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Function to validate Thai phone number
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

-- Function to update profile completion status (updated without national_id from migration 012)
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

-- ============================================================================
-- TRIGGERS
-- ============================================================================

-- Trigger for updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Trigger for profile completion
CREATE TRIGGER trigger_update_profile_completion
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_profile_completion();

-- ============================================================================
-- CONSTRAINTS
-- ============================================================================

-- Add constraints for data validation
DO $$
BEGIN
    -- Add constraints only if they don't exist
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

-- ============================================================================
-- SAMPLE DATA (from migrations 005, 006, 007)
-- ============================================================================

-- Insert sample activity for testing (with fixed typo from migration 007)
INSERT INTO activities (name, description, start_date, end_date, status, max_participants) 
VALUES (
    'Ananped 8M Subscribers Celebration',
    'Celebrate 8 million subscribers with voting activity',
    NOW(),
    NOW() + INTERVAL '30 days',
    'active',
    1000000
) ON CONFLICT DO NOTHING;

-- Insert sample teams A-H
INSERT INTO teams (activity_id, name, display_name, description, image_url) 
SELECT 
    a.id,
    team_name,
    'Team ' || team_name,
    'Team ' || team_name || ' for the celebration',
    CASE team_name
        WHEN 'A' THEN 'https://cdn.jsdelivr.net/npm/twemoji@11.3.0/2/svg/1f1e6.svg'
        WHEN 'B' THEN 'https://cdn.jsdelivr.net/npm/twemoji@11.3.0/2/svg/1f1e7.svg'  
        WHEN 'C' THEN 'https://cdn.jsdelivr.net/npm/twemoji@11.3.0/2/svg/1f1e8.svg'
        WHEN 'D' THEN 'https://cdn.jsdelivr.net/npm/twemoji@11.3.0/2/svg/1f1e9.svg'
        WHEN 'E' THEN 'https://cdn.jsdelivr.net/npm/twemoji@11.3.0/2/svg/1f1ea.svg'
        WHEN 'F' THEN 'https://cdn.jsdelivr.net/npm/twemoji@11.3.0/2/svg/1f1eb.svg'
        WHEN 'G' THEN 'https://cdn.jsdelivr.net/npm/twemoji@11.3.0/2/svg/1f1ec.svg'
        WHEN 'H' THEN 'https://cdn.jsdelivr.net/npm/twemoji@11.3.0/2/svg/1f1ed.svg'
    END
FROM activities a
CROSS JOIN (VALUES ('A'), ('B'), ('C'), ('D'), ('E'), ('F'), ('G'), ('H')) AS t(team_name)
WHERE a.name = 'Ananped 8M Subscribers Celebration'
ON CONFLICT DO NOTHING;

-- Insert initial terms and PDPA versions
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
4. ท่านมีสิทธิ์ขอดูข้อมูล แก้ไข หรือลบข้อมูลส่วนบุคคล
5. หากมีข้อสงสัย สามารถติดต่อเราได้ตามช่องทางที่ระบุ', TRUE
WHERE NOT EXISTS (SELECT 1 FROM terms_versions WHERE version = '1.0' AND type = 'pdpa');

-- ============================================================================
-- ROW LEVEL SECURITY (RLS) - Neon Compatible
-- ============================================================================

-- Enable RLS on tables
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE activities ENABLE ROW LEVEL SECURITY;
ALTER TABLE teams ENABLE ROW LEVEL SECURITY;
ALTER TABLE votes ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE terms_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_terms_acceptance ENABLE ROW LEVEL SECURITY;

-- Basic RLS policies for Neon (simplified for standard PostgreSQL)
-- Drop existing policies first to avoid conflicts
DROP POLICY IF EXISTS "Users can read own data" ON users;
DROP POLICY IF EXISTS "Anyone can read activities" ON activities;
DROP POLICY IF EXISTS "Anyone can read teams" ON teams;
DROP POLICY IF EXISTS "Users can create votes" ON votes;
DROP POLICY IF EXISTS "Users can read votes" ON votes;
DROP POLICY IF EXISTS "Anyone can read active terms" ON terms_versions;
DROP POLICY IF EXISTS "Users can create terms acceptance" ON user_terms_acceptance;

-- Create new policies
CREATE POLICY "Users can read own data" ON users
    FOR SELECT USING (google_id = current_user);

CREATE POLICY "Anyone can read activities" ON activities
    FOR SELECT USING (true);

CREATE POLICY "Anyone can read teams" ON teams
    FOR SELECT USING (true);

CREATE POLICY "Users can create votes" ON votes
    FOR INSERT WITH CHECK (true);

CREATE POLICY "Users can read votes" ON votes
    FOR SELECT USING (true);

CREATE POLICY "Anyone can read active terms" ON terms_versions
    FOR SELECT USING (active = TRUE);

CREATE POLICY "Users can create terms acceptance" ON user_terms_acceptance
    FOR INSERT WITH CHECK (true);

-- ============================================================================
-- GRANT PERMISSIONS
-- ============================================================================

-- Grant basic permissions (adjust as needed for your Neon setup)
GRANT USAGE ON SCHEMA public TO PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO PUBLIC;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO PUBLIC;

-- ============================================================================
-- LOG MIGRATION COMPLETION
-- ============================================================================

-- Log all migrations as applied
INSERT INTO schema_migrations (version, applied_at) VALUES
('001_initial_schema', NOW()),
('002_user_profile_pdpa_updates', NOW()),
('003_fix_user_schema', NOW()),
('004_add_oauth_tokens', NOW()),
('005_create_hardcoded_activity', NOW()),
('006_create_hardcoded_teams', NOW()),
('007_fix_annanped_typo', NOW()),
('008_fix_session_token_length', NOW()),
('009_fix_profile_completion_logic', NOW()),
('010_add_activity_rules_acceptance', NOW()),
('011_remove_national_id', NOW()),
('012_fix_profile_completion_trigger', NOW()),
('neon_migration_complete', NOW())
ON CONFLICT (version) DO NOTHING;

-- Add helpful comments
COMMENT ON TABLE users IS 'User profiles - national_id removed, only first_name, last_name, phone required for profile completion';
COMMENT ON COLUMN users.google_access_token IS 'Google OAuth access token for YouTube API access';
COMMENT ON COLUMN users.google_refresh_token IS 'Google OAuth refresh token for token renewal';
COMMENT ON COLUMN users.google_token_expiry IS 'Expiry time for the Google OAuth access token';
COMMENT ON COLUMN users.youtube_channel_id IS 'Users YouTube channel ID for subscription verification';
COMMENT ON FUNCTION update_profile_completion() IS 'Updates profile completion status based on first_name, last_name, and phone only (national_id removed)';

-- Migration complete message
DO $$
BEGIN
    RAISE NOTICE '🎉 Neon Database Migration Complete!';
    RAISE NOTICE '📊 All 12 migrations have been consolidated and applied';
    RAISE NOTICE '✅ Database is ready for the YouTube Activity Backend';
END
$$;