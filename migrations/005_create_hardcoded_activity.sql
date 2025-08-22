-- Migration 005: Create Hardcoded Activity for One-Time Event
-- This creates the hardcoded "active" activity record that the voting system expects

-- Insert the hardcoded activity record if it doesn't exist
INSERT INTO activities (id, name, title, description, channel_id, start_date, end_date, status, max_participants)
VALUES (
    '550e8400-e29b-41d4-a716-446655440000',
    'Ananped 10M Voting',
    'Ananped 10M โหวตทีมที่คุณชื่นชอบ',
    'ร่วมเฉลิมฉลองกับกิจกรรมพิเศษ 10 ล้าน Subscribers!',
    'UC-chqi3Gpb4F7yBqedlnq5g', -- Ananped channel ID
    '2025-08-22 00:00:00'::timestamp,
    '2025-12-31 23:59:59'::timestamp,
    'active',
    10000
)
ON CONFLICT (id) DO NOTHING;

-- Verify the activity was created
SELECT id, name, title, description, status FROM activities WHERE id = '550e8400-e29b-41d4-a716-446655440000';