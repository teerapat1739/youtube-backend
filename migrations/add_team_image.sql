-- Add image_filename column to teams table
ALTER TABLE teams 
ADD COLUMN IF NOT EXISTS image_filename VARCHAR(255);

-- Update existing teams with their corresponding image filenames
UPDATE teams SET image_filename = 'team-1.png' WHERE code = 'team-alpha';
UPDATE teams SET image_filename = 'team-2.png' WHERE code = 'team-beta';
UPDATE teams SET image_filename = 'team-3.png' WHERE code = 'team-gamma';
UPDATE teams SET image_filename = 'team-4.png' WHERE code = 'team-delta';
UPDATE teams SET image_filename = 'team-5.png' WHERE code = 'team-epsilon';

-- Add comment to document the column
COMMENT ON COLUMN teams.image_filename IS 'Filename of the team image stored in frontend assets';

-- Refresh the materialized view to include the new column
DROP MATERIALIZED VIEW IF EXISTS vote_summary CASCADE;

CREATE MATERIALIZED VIEW vote_summary AS
SELECT 
    t.id,
    t.code,
    t.name,
    t.description,
    t.icon,
    t.image_filename,
    t.member_count,
    COUNT(v.id) as vote_count,
    MAX(v.created_at) as last_vote_at
FROM teams t
LEFT JOIN votes v ON t.id = v.team_id
WHERE t.is_active = true
GROUP BY t.id, t.code, t.name, t.description, t.icon, t.image_filename, t.member_count;

-- Recreate the unique index
CREATE UNIQUE INDEX idx_vote_summary_team_id ON vote_summary(id);

-- Refresh the view with current data
REFRESH MATERIALIZED VIEW vote_summary;