-- Migration 007: Fix typo Ananped → Ananped
-- This corrects the spelling throughout the database

-- Update activities table
UPDATE activities 
SET 
    title = REPLACE(title, 'Ananped', 'Ananped'),
    description = REPLACE(description, 'Ananped', 'Ananped'),
    name = REPLACE(name, 'Ananped', 'Ananped')
WHERE 
    title LIKE '%Ananped%' OR 
    description LIKE '%Ananped%' OR 
    name LIKE '%Ananped%';

-- Update teams table  
UPDATE teams
SET 
    description = REPLACE(description, 'Ananped', 'Ananped'),
    display_name = REPLACE(display_name, 'Ananped', 'Ananped')
WHERE 
    description LIKE '%Ananped%' OR 
    display_name LIKE '%Ananped%';

-- Verify changes
SELECT 'ACTIVITIES' as table_name, id, name, title, description FROM activities WHERE title LIKE '%Ananped%' OR description LIKE '%Ananped%'
UNION ALL
SELECT 'TEAMS' as table_name, id, name, display_name, description FROM teams WHERE description LIKE '%Ananped%' OR display_name LIKE '%Ananped%';