-- Fix user schema inconsistencies and add PDPA/Terms fields
-- This migration aligns the schema with the models.User struct

-- First, rename existing columns to match models
ALTER TABLE users RENAME COLUMN id_card_number TO national_id;
ALTER TABLE users RENAME COLUMN phone_number TO phone;

-- Make first_name and last_name nullable for initial user creation
ALTER TABLE users ALTER COLUMN first_name DROP NOT NULL;
ALTER TABLE users ALTER COLUMN last_name DROP NOT NULL;
ALTER TABLE users ALTER COLUMN national_id DROP NOT NULL;
ALTER TABLE users ALTER COLUMN phone DROP NOT NULL;

-- Add PDPA and terms acceptance fields if they don't exist
ALTER TABLE users ADD COLUMN IF NOT EXISTS terms_accepted BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS terms_version VARCHAR(50);
ALTER TABLE users ADD COLUMN IF NOT EXISTS pdpa_accepted BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS pdpa_version VARCHAR(50);
ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_completed BOOLEAN DEFAULT FALSE;

-- Add unique constraint on national_id only when it's not null
DROP INDEX IF EXISTS users_national_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS users_national_id_unique ON users(national_id) WHERE national_id IS NOT NULL;

-- Create terms_versions table for version management
CREATE TABLE IF NOT EXISTS terms_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    version VARCHAR(50) NOT NULL,
    type VARCHAR(10) NOT NULL CHECK (type IN ('terms', 'pdpa')),
    content TEXT NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(type, version)
);

-- Insert default terms versions
INSERT INTO terms_versions (version, type, content, active) VALUES
('1.0', 'terms', 'Default Terms and Conditions', TRUE),
('1.0', 'pdpa', 'Default Privacy Policy', TRUE)
ON CONFLICT (type, version) DO NOTHING;

-- Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_terms_accepted ON users(terms_accepted);
CREATE INDEX IF NOT EXISTS idx_users_pdpa_accepted ON users(pdpa_accepted);
CREATE INDEX IF NOT EXISTS idx_users_profile_completed ON users(profile_completed);
CREATE INDEX IF NOT EXISTS idx_terms_versions_type_active ON terms_versions(type, active);

-- Create user_terms_acceptance table for audit trail
CREATE TABLE IF NOT EXISTS user_terms_acceptance (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    terms_version VARCHAR(50),
    pdpa_version VARCHAR(50),
    accepted_at TIMESTAMP DEFAULT NOW(),
    ip_address INET,
    user_agent TEXT,
    UNIQUE(user_id, terms_version, pdpa_version)
);

-- Add index for user_terms_acceptance
CREATE INDEX IF NOT EXISTS idx_user_terms_acceptance_user ON user_terms_acceptance(user_id);
CREATE INDEX IF NOT EXISTS idx_user_terms_acceptance_accepted_at ON user_terms_acceptance(accepted_at);

-- Log completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('003_fix_user_schema', NOW()) 
ON CONFLICT (version) DO NOTHING;