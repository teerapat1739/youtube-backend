package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Get database URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	// Get command
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go [drop|up|seed|cleanup|phone-migration|welcome-tracking]")
		os.Exit(1)
	}

	command := os.Args[1]

	// Connect to database
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	switch command {
	case "drop":
		if err := dropTables(ctx, conn); err != nil {
			log.Fatalf("Failed to drop tables: %v", err)
		}
		fmt.Println("✅ All tables dropped successfully")

	case "up":
		if err := createTables(ctx, conn); err != nil {
			log.Fatalf("Failed to create tables: %v", err)
		}
		fmt.Println("✅ All tables created successfully")

	case "seed":
		if err := seedData(ctx, conn); err != nil {
			log.Fatalf("Failed to seed data: %v", err)
		}
		fmt.Println("✅ Data seeded successfully")

	case "cleanup":
		if err := dropUnusedTables(ctx, conn); err != nil {
			log.Fatalf("Failed to cleanup unused tables: %v", err)
		}
		fmt.Println("✅ Unused tables cleaned up successfully")

	case "phone-migration":
		if err := runPhoneMigration(ctx, conn); err != nil {
			log.Fatalf("Failed to run phone migration: %v", err)
		}
		fmt.Println("✅ Phone number migration completed successfully")

	case "welcome-tracking":
		if err := runWelcomeTrackingMigration(ctx, conn); err != nil {
			log.Fatalf("Failed to run welcome tracking migration: %v", err)
		}
		fmt.Println("✅ Welcome tracking migration completed successfully")

	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Usage: go run main.go [drop|up|seed|cleanup|phone-migration|welcome-tracking]")
		os.Exit(1)
	}
}

func dropTables(ctx context.Context, conn *pgx.Conn) error {
	queries := []string{
		`DROP MATERIALIZED VIEW IF EXISTS vote_summary CASCADE`,
		`DROP TABLE IF EXISTS votes CASCADE`,
		`DROP TABLE IF EXISTS teams CASCADE`,
	}

	for _, query := range queries {
		if _, err := conn.Exec(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
		fmt.Printf("  Dropped: %s\n", query)
	}

	return nil
}

func dropUnusedTables(ctx context.Context, conn *pgx.Conn) error {
	// Drop unused tables that are not part of the voting system
	queries := []string{
		`DROP TABLE IF EXISTS user_sessions CASCADE`,
		`DROP TABLE IF EXISTS user_terms_acceptance CASCADE`,
		`DROP TABLE IF EXISTS profiles CASCADE`,
		`DROP TABLE IF EXISTS activities CASCADE`,
		`DROP TABLE IF EXISTS terms_versions CASCADE`,
		`DROP TABLE IF EXISTS users CASCADE`,
	}

	for _, query := range queries {
		if _, err := conn.Exec(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
		fmt.Printf("  Dropped unused table: %s\n", query)
	}

	return nil
}

func createTables(ctx context.Context, conn *pgx.Conn) error {
	queries := []string{
		// Create teams table
		`CREATE TABLE IF NOT EXISTS teams (
			id SERIAL PRIMARY KEY,
			code VARCHAR(50) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			icon VARCHAR(10),
			member_count INTEGER DEFAULT 0,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,

		// Create votes table with PDPA compliance fields
		`CREATE TABLE IF NOT EXISTS votes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			vote_id VARCHAR(20) UNIQUE NOT NULL,
			user_id VARCHAR(255) NOT NULL,
			team_id INTEGER REFERENCES teams(id) ON DELETE CASCADE,
			voter_name VARCHAR(255) NOT NULL,
			voter_email VARCHAR(255) NOT NULL,
			voter_phone VARCHAR(20),
			ip_address INET,
			user_agent TEXT,
			consent_timestamp TIMESTAMP,
			consent_ip INET,
			privacy_policy_version VARCHAR(10),
			pdpa_consent BOOLEAN DEFAULT false,
			marketing_consent BOOLEAN DEFAULT false,
			data_retention_until TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(user_id)
		)`,

		// Create materialized view for vote summary
		`CREATE MATERIALIZED VIEW IF NOT EXISTS vote_summary AS
		SELECT 
			t.id,
			t.code,
			t.name,
			t.description,
			t.icon,
			t.member_count,
			COUNT(v.id) as vote_count,
			MAX(v.created_at) as last_vote_at
		FROM teams t
		LEFT JOIN votes v ON t.id = v.team_id
		WHERE t.is_active = true
		GROUP BY t.id, t.code, t.name, t.description, t.icon, t.member_count`,

		// Create indexes
		`CREATE INDEX IF NOT EXISTS idx_votes_user_id ON votes(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_votes_team_id ON votes(team_id)`,
		`CREATE INDEX IF NOT EXISTS idx_votes_created_at ON votes(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_teams_active ON teams(is_active)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_vote_summary_team_id ON vote_summary(id)`,
	}

	for _, query := range queries {
		if _, err := conn.Exec(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %w\nQuery: %s", err, query)
		}
		fmt.Printf("  Created: %s\n", getTableName(query))
	}

	return nil
}

func seedData(ctx context.Context, conn *pgx.Conn) error {
	// Insert team data
	query := `
		INSERT INTO teams (code, name, description, icon, member_count) VALUES
		('team-alpha', 'ทีม Alpha', 'นวัตกรรมเพื่ออนาคต', '🚀', 45),
		('team-beta', 'ทีม Beta', 'ความคิดสร้างสรรค์ไร้ขีดจำกัด', '🎨', 38),
		('team-gamma', 'ทีม Gamma', 'พลังแห่งความร่วมมือ', '🤝', 52),
		('team-delta', 'ทีม Delta', 'ความเป็นเลิศในทุกมิติ', '⭐', 41),
		('team-epsilon', 'ทีม Epsilon', 'สู่ความยั่งยืน', '🌱', 33)
		ON CONFLICT (code) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			icon = EXCLUDED.icon,
			member_count = EXCLUDED.member_count,
			updated_at = NOW()
	`

	if _, err := conn.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to seed teams: %w", err)
	}

	fmt.Println("  Seeded 5 teams")

	// Refresh materialized view
	if _, err := conn.Exec(ctx, "REFRESH MATERIALIZED VIEW vote_summary"); err != nil {
		return fmt.Errorf("failed to refresh materialized view: %w", err)
	}

	fmt.Println("  Refreshed materialized view")

	return nil
}

func getTableName(query string) string {
	if len(query) > 50 {
		return query[:50] + "..."
	}
	return query
}

func runPhoneMigration(ctx context.Context, conn *pgx.Conn) error {
	// Read the migration SQL file
	sqlFile := "migrations/phone_standardization.sql"
	if _, err := os.Stat(sqlFile); os.IsNotExist(err) {
		return fmt.Errorf("migration file not found: %s", sqlFile)
	}

	sqlBytes, err := ioutil.ReadFile(sqlFile)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Execute the migration SQL
	_, err = conn.Exec(ctx, string(sqlBytes))
	if err != nil {
		return fmt.Errorf("failed to execute phone migration: %w", err)
	}

	fmt.Println("  ✅ Phone numbers normalized and unique constraint added")
	fmt.Println("  ✅ Duplicate records removed (kept earliest vote per phone)")
	fmt.Println("  ✅ Index added for better phone lookup performance")

	return nil
}

func runWelcomeTrackingMigration(ctx context.Context, conn *pgx.Conn) error {
	// Read the migration SQL file
	sqlFile := "migrations/add_welcome_tracking.sql"
	if _, err := os.Stat(sqlFile); os.IsNotExist(err) {
		return fmt.Errorf("migration file not found: %s", sqlFile)
	}

	sqlBytes, err := ioutil.ReadFile(sqlFile)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Execute the migration SQL
	_, err = conn.Exec(ctx, string(sqlBytes))
	if err != nil {
		return fmt.Errorf("failed to execute welcome tracking migration: %w", err)
	}

	fmt.Println("  ✅ Welcome tracking columns added to votes table")
	fmt.Println("  ✅ Index on welcome_accepted column created")
	fmt.Println("  ✅ Composite index on user_id and welcome_accepted created")
	fmt.Println("  ✅ Column comments added for documentation")

	return nil
}