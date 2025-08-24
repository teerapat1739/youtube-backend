-- Migration 005: Create Hardcoded Activity for One-Time Event
-- This creates the hardcoded "active" activity record that the voting system expects

-- Insert the hardcoded activity record if it doesn't exist
INSERT INTO activities (id, name, title, description, channel_id, start_date, end_date, status, max_participants)
VALUES (
    '550e8400-e29b-41d4-a716-446655440000',
    'Ananped 8M Celebration',
    'Ananped 8M Celebration - Vote for Your Favorite Team!',
    'Join the 14-day celebration event for Ananped reaching 8 million subscribers!',
    'UC-chqi3Gpb4F7yBqedlnq5g', -- Ananped channel ID
    '2024-09-05 00:00:00'::timestamp,
    '2024-09-19 23:59:59'::timestamp,
    'active',
    50000
)
ON CONFLICT (id) DO NOTHING;

-- Log completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('005_create_hardcoded_activity', NOW()) 
ON CONFLICT (version) DO NOTHING;

-- Verify the activity was created
SELECT id, name, title, description, status FROM activities WHERE id = '550e8400-e29b-41d4-a716-446655440000';