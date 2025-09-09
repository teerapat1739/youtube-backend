-- Migration: Create visitor snapshots table for periodic visitor statistics storage
-- This migration creates the table to store periodic snapshots of visitor statistics from Redis

BEGIN;

-- Create visitor_snapshots table
CREATE TABLE IF NOT EXISTS visitor_snapshots (
    id SERIAL PRIMARY KEY,
    total_visits BIGINT NOT NULL DEFAULT 0,
    daily_visits BIGINT NOT NULL DEFAULT 0,
    unique_visits BIGINT NOT NULL DEFAULT 0,
    snapshot_date DATE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create index on snapshot_date for efficient querying of historical data
CREATE INDEX IF NOT EXISTS idx_visitor_snapshots_snapshot_date ON visitor_snapshots(snapshot_date DESC);

-- Create unique index to prevent duplicate snapshots for the same date
CREATE UNIQUE INDEX IF NOT EXISTS idx_visitor_snapshots_unique_date ON visitor_snapshots(snapshot_date);

-- Add comments for documentation
COMMENT ON TABLE visitor_snapshots IS 'Periodic snapshots of visitor statistics from Redis cache';
COMMENT ON COLUMN visitor_snapshots.id IS 'Primary key for the snapshot record';
COMMENT ON COLUMN visitor_snapshots.total_visits IS 'Total number of visits recorded';
COMMENT ON COLUMN visitor_snapshots.daily_visits IS 'Number of visits for the snapshot date';
COMMENT ON COLUMN visitor_snapshots.unique_visits IS 'Number of unique visitors recorded';
COMMENT ON COLUMN visitor_snapshots.snapshot_date IS 'Date for which this snapshot was taken';
COMMENT ON COLUMN visitor_snapshots.created_at IS 'Timestamp when this snapshot was created';

-- Log the migration completion
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('create_visitor_snapshots_001', NOW())
ON CONFLICT (version) DO NOTHING;

COMMIT;

