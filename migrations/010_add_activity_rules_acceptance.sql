-- Migration 010: Add Activity Rules Acceptance Fields
-- This migration adds fields to track activity rules acceptance for users

-- Start transaction
BEGIN;

-- Add columns for activity rules acceptance to users table
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS activity_rules_accepted BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS activity_rules_accepted_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS activity_rules_version VARCHAR(10) DEFAULT '1.0';

-- Create activity_rules_versions table to store activity rules content
CREATE TABLE IF NOT EXISTS activity_rules_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    version VARCHAR(10) NOT NULL UNIQUE,
    title VARCHAR(200) NOT NULL DEFAULT 'กติกาเข้าร่วมกิจกรรม',
    description TEXT,
    rules_content JSONB NOT NULL, -- Store structured rules data as JSON
    active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create user_activity_rules_acceptance table for audit trail
CREATE TABLE IF NOT EXISTS user_activity_rules_acceptance (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rules_version VARCHAR(10) NOT NULL,
    accepted_at TIMESTAMP DEFAULT NOW(),
    ip_address VARCHAR(45),
    user_agent TEXT,
    UNIQUE(user_id, rules_version)
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_activity_rules_accepted ON users(activity_rules_accepted);
CREATE INDEX IF NOT EXISTS idx_users_activity_rules_accepted_at ON users(activity_rules_accepted_at);
CREATE INDEX IF NOT EXISTS idx_users_activity_rules_version ON users(activity_rules_version);
CREATE INDEX IF NOT EXISTS idx_activity_rules_versions_active ON activity_rules_versions(active);
CREATE INDEX IF NOT EXISTS idx_user_activity_rules_user_id ON user_activity_rules_acceptance(user_id);
CREATE INDEX IF NOT EXISTS idx_user_activity_rules_accepted_at ON user_activity_rules_acceptance(accepted_at);

-- Insert initial activity rules content (version 1.0)
INSERT INTO activity_rules_versions (version, title, description, rules_content, active) 
SELECT 
    '1.0',
    'กติกาเข้าร่วมกิจกรรม',
    'ฉลอง 10 ล้านผู้ติดตาม! ร่วมโหวตเลือกทีมที่คุณชื่นชอบและลุ้นรับของรางวัลมากมาย',
    '{
        "objective": {
            "title": "🎯 เป้าหมายของกิจกรรม",
            "content": [
                "🎉 ฉลอง 10 ล้านผู้ติดตาม!",
                "ร่วมโหวตเลือกทีมที่คุณชื่นชอบและลุ้นรับของรางวัลมากมาย",
                "เป็นส่วนหนึ่งของชุมชน Ananped ที่ยิ่งใหญ่!"
            ]
        },
        "participation": {
            "title": "📝 วิธีการเข้าร่วม",
            "rules": [
                {
                    "title": "ติดตามช่อง",
                    "description": "ต้องเป็น Subscriber ของช่อง Ananped"
                },
                {
                    "title": "กรอกข้อมูล",
                    "description": "กรอกข้อมูลส่วนตัวให้ครบถ้วนและถูกต้อง"
                },
                {
                    "title": "โหวต",
                    "description": "เลือกโหวตทีมที่คุณชื่นชอบ (1 ครั้งต่อบัญชี)"
                },
                {
                    "title": "รอผลลัพธ์",
                    "description": "ติดตามผลการโหวตและการประกาศรางวัล"
                }
            ]
        },
        "prizes": {
            "title": "🎁 ของรางวัล",
            "items": [
                {
                    "icon": "🏆",
                    "rank": "รางวัลที่ 1",
                    "prize": "ไอโฟนรุ่นล่าสุด (1 รางวัล)"
                },
                {
                    "icon": "🥈",
                    "rank": "รางวัลที่ 2", 
                    "prize": "เงินสด 50,000 บาท (3 รางวัล)"
                },
                {
                    "icon": "🥉",
                    "rank": "รางวัลที่ 3",
                    "prize": "กิฟต์การ์ด 10,000 บาท (10 รางวัล)"
                },
                {
                    "icon": "🎊",
                    "rank": "รางวัลปลอบใจ",
                    "prize": "สติกเกอร์ Ananped (100 รางวัล)"
                }
            ]
        },
        "important_rules": {
            "title": "⚠️ กติกาสำคัญ",
            "rules": [
                "หนึ่งบัญชีหนึ่งโหวต: สามารถโหวตได้เพียงครั้งเดียวต่อบัญชี Google",
                "ข้อมูลจริง: ข้อมูลที่กรอกต้องเป็นข้อมูลจริงและถูกต้อง",
                "การตรวจสอบ: จะมีการตรวจสอบการติดตามช่องก่อนนับคะแนน",
                "ไม่มีการแลกเปลี่ยน: ของรางวัลไม่สามารถแลกเปลี่ยนเป็นเงินสดได้",
                "การตัดสิน: การตัดสินของทางช่องถือเป็นที่สุด"
            ]
        },
        "timeline": {
            "title": "📅 กำหนดการ",
            "events": [
                {
                    "date": "วันนี้ - 31 ธ.ค. 2567",
                    "event": "เปิดรับการโหวต"
                },
                {
                    "date": "1 ม.ค. 2568",
                    "event": "ปิดการโหวต"
                },
                {
                    "date": "5 ม.ค. 2568", 
                    "event": "ประกาศผลรางวัล"
                },
                {
                    "date": "10 ม.ค. 2568",
                    "event": "จัดส่งรางวัล"
                }
            ]
        }
    }',
    TRUE
WHERE NOT EXISTS (SELECT 1 FROM activity_rules_versions WHERE version = '1.0');

-- Enable RLS for new tables
ALTER TABLE activity_rules_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_activity_rules_acceptance ENABLE ROW LEVEL SECURITY;

-- Create RLS policies for activity rules tables
-- Anyone can read active activity rules versions
DROP POLICY IF EXISTS "Anyone can read active activity rules" ON activity_rules_versions;
CREATE POLICY "Anyone can read active activity rules" ON activity_rules_versions
    FOR SELECT USING (active = TRUE);

-- Users can read their own activity rules acceptance records  
DROP POLICY IF EXISTS "Users can read own activity rules acceptance" ON user_activity_rules_acceptance;
CREATE POLICY "Users can read own activity rules acceptance" ON user_activity_rules_acceptance
    FOR SELECT USING (user_id::text = auth.uid()::text);

-- Users can create activity rules acceptance records
DROP POLICY IF EXISTS "Users can create activity rules acceptance" ON user_activity_rules_acceptance;
CREATE POLICY "Users can create activity rules acceptance" ON user_activity_rules_acceptance
    FOR INSERT WITH CHECK (true);

-- Create function to update activity rules acceptance timestamp
CREATE OR REPLACE FUNCTION update_activity_rules_acceptance()
RETURNS TRIGGER AS $$
BEGIN
    -- If activity_rules_accepted is being set to true and it wasn't true before
    IF NEW.activity_rules_accepted = TRUE AND (OLD.activity_rules_accepted IS NULL OR OLD.activity_rules_accepted = FALSE) THEN
        NEW.activity_rules_accepted_at := NOW();
    END IF;
    
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for activity rules acceptance
DROP TRIGGER IF EXISTS update_activity_rules_acceptance_trigger ON users;
CREATE TRIGGER update_activity_rules_acceptance_trigger
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_activity_rules_acceptance();

-- Grant permissions for new tables and functions
GRANT USAGE ON SCHEMA public TO anon, authenticated;
GRANT ALL ON TABLE activity_rules_versions TO anon, authenticated;
GRANT ALL ON TABLE user_activity_rules_acceptance TO anon, authenticated;
GRANT EXECUTE ON FUNCTION update_activity_rules_acceptance() TO anon, authenticated;

-- Commit transaction
COMMIT;

-- Log completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('010_add_activity_rules_acceptance', NOW()) 
ON CONFLICT (version) DO NOTHING;

-- Display migration completion message
DO $$
BEGIN
    RAISE NOTICE 'Migration 010_add_activity_rules_acceptance.sql completed successfully!';
    RAISE NOTICE 'Added: activity_rules_accepted, activity_rules_versions table, and audit trail';
END
$$;