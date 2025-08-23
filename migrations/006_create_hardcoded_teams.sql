-- Migration 006: Create Hardcoded Teams for One-Time Event
-- This creates the 6 hardcoded teams (A, B, C, D, E, F) that match the Go code

-- Insert the 6 hardcoded teams if they don't exist
INSERT INTO teams (id, name, display_name, description, activity_id) VALUES
('550e8400-e29b-41d4-a716-446655440001', 'A', 'Team Crimson', 'Team Crimson for the Ananped 8M celebration', '550e8400-e29b-41d4-a716-446655440000'),
('550e8400-e29b-41d4-a716-446655440002', 'B', 'Team Azure', 'Team Azure for the Ananped 8M celebration', '550e8400-e29b-41d4-a716-446655440000'),
('550e8400-e29b-41d4-a716-446655440003', 'C', 'Team Golden', 'Team Golden for the Ananped 8M celebration', '550e8400-e29b-41d4-a716-446655440000'),
('550e8400-e29b-41d4-a716-446655440004', 'D', 'Team Emerald', 'Team Emerald for the Ananped 8M celebration', '550e8400-e29b-41d4-a716-446655440000'),
('550e8400-e29b-41d4-a716-446655440005', 'E', 'Team Purple', 'Team Purple for the Ananped 8M celebration', '550e8400-e29b-41d4-a716-446655440000'),
('550e8400-e29b-41d4-a716-446655440006', 'F', 'Team Silver', 'Team Silver for the Ananped 8M celebration', '550e8400-e29b-41d4-a716-446655440000')
ON CONFLICT (id) DO NOTHING;

-- Verify the teams were created
SELECT id, name, description, activity_id FROM teams WHERE activity_id = '550e8400-e29b-41d4-a716-446655440000' ORDER BY name;