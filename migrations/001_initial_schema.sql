-- Initial database schema for Activity Landing Page
-- Run this in your Supabase SQL editor

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    google_id VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    id_card_number VARCHAR(20) UNIQUE NOT NULL,
    phone_number VARCHAR(20) NOT NULL,
    youtube_subscribed BOOLEAN DEFAULT FALSE,
    subscription_verified_at TIMESTAMP,
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

-- User sessions table
CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    session_token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_votes_activity_team ON votes(activity_id, team_id);
CREATE INDEX IF NOT EXISTS idx_votes_user_activity ON votes(user_id, activity_id);
CREATE INDEX IF NOT EXISTS idx_votes_created_at ON votes(created_at);
CREATE INDEX IF NOT EXISTS idx_teams_activity ON teams(activity_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_token ON user_sessions(session_token);

-- Insert sample activity for testing
INSERT INTO activities (name, description, start_date, end_date, status, max_participants) 
VALUES (
    'Annanped 8M Subscribers Celebration',
    'Celebrate 8 million subscribers with voting activity',
    NOW(),
    NOW() + INTERVAL '30 days',
    'active',
    1000000
) ON CONFLICT DO NOTHING;

-- Insert sample teams A-H
INSERT INTO teams (activity_id, name, display_name, description) 
SELECT 
    a.id,
    team_name,
    'Team ' || team_name,
    'Team ' || team_name || ' for the celebration'
FROM activities a
CROSS JOIN (VALUES ('A'), ('B'), ('C'), ('D'), ('E'), ('F'), ('G'), ('H')) AS t(team_name)
WHERE a.name = 'Annanped 8M Subscribers Celebration'
ON CONFLICT DO NOTHING;

-- Enable Row Level Security (RLS)
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE activities ENABLE ROW LEVEL SECURITY;
ALTER TABLE teams ENABLE ROW LEVEL SECURITY;
ALTER TABLE votes ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_sessions ENABLE ROW LEVEL SECURITY;

-- Create RLS policies
-- Users can read their own data
CREATE POLICY "Users can read own data" ON users
    FOR SELECT USING (auth.uid()::text = google_id);

-- Users can update their own data
CREATE POLICY "Users can update own data" ON users
    FOR UPDATE USING (auth.uid()::text = google_id);

-- Anyone can read activities
CREATE POLICY "Anyone can read activities" ON activities
    FOR SELECT USING (true);

-- Anyone can read teams
CREATE POLICY "Anyone can read teams" ON teams
    FOR SELECT USING (true);

-- Users can create votes
CREATE POLICY "Users can create votes" ON votes
    FOR INSERT WITH CHECK (true);

-- Users can read votes
CREATE POLICY "Users can read votes" ON votes
    FOR SELECT USING (true);

-- Users can read their own sessions
CREATE POLICY "Users can read own sessions" ON user_sessions
    FOR SELECT USING (auth.uid()::text = user_id::text);

-- Users can create sessions
CREATE POLICY "Users can create sessions" ON user_sessions
    FOR INSERT WITH CHECK (true);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Grant necessary permissions
GRANT USAGE ON SCHEMA public TO anon, authenticated;
GRANT ALL ON ALL TABLES IN SCHEMA public TO anon, authenticated;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO anon, authenticated;
GRANT ALL ON ALL FUNCTIONS IN SCHEMA public TO anon, authenticated;
