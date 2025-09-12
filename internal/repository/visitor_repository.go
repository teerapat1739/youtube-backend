package repository

import (
	"context"
	"fmt"
	"time"

	"be-v2/internal/domain"
	"be-v2/pkg/database"
	"github.com/jackc/pgx/v5"
)

// visitorRepository handles visitor snapshot operations with PostgreSQL
type visitorRepository struct {
	db *database.PostgresDB
}

// NewVisitorRepository creates a new visitor repository
func NewVisitorRepository(db *database.PostgresDB) VisitorRepository {
	return &visitorRepository{
		db: db,
	}
}

// CreateSnapshot creates a new visitor snapshot in the database
func (r *visitorRepository) CreateSnapshot(ctx context.Context, snapshot *domain.VisitorSnapshot) error {
	query := `
		INSERT INTO visitor_snapshots (total_visits, daily_visits, unique_visits, snapshot_date, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (snapshot_date) DO UPDATE SET
			total_visits = EXCLUDED.total_visits,
			daily_visits = EXCLUDED.daily_visits,
			unique_visits = EXCLUDED.unique_visits,
			created_at = EXCLUDED.created_at
		RETURNING id, created_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		snapshot.TotalVisits,
		snapshot.DailyVisits,
		snapshot.UniqueVisits,
		snapshot.SnapshotDate,
		snapshot.CreatedAt,
	).Scan(&snapshot.ID, &snapshot.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create visitor snapshot: %w", err)
	}

	return nil
}

// GetLatestSnapshot retrieves the most recent visitor snapshot
func (r *visitorRepository) GetLatestSnapshot(ctx context.Context) (*domain.VisitorSnapshot, error) {
	query := `
		SELECT id, total_visits, daily_visits, unique_visits, snapshot_date, created_at
		FROM visitor_snapshots
		ORDER BY snapshot_date DESC, created_at DESC
		LIMIT 1
	`

	snapshot := &domain.VisitorSnapshot{}
	err := r.db.GetReadPool().QueryRow(ctx, query).Scan(
		&snapshot.ID,
		&snapshot.TotalVisits,
		&snapshot.DailyVisits,
		&snapshot.UniqueVisits,
		&snapshot.SnapshotDate,
		&snapshot.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			// No snapshots exist yet, return nil without error
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest visitor snapshot: %w", err)
	}

	return snapshot, nil
}

// GetSnapshotByDate retrieves a visitor snapshot for a specific date
func (r *visitorRepository) GetSnapshotByDate(ctx context.Context, date time.Time) (*domain.VisitorSnapshot, error) {
	query := `
		SELECT id, total_visits, daily_visits, unique_visits, snapshot_date, created_at
		FROM visitor_snapshots
		WHERE snapshot_date = $1
	`

	snapshot := &domain.VisitorSnapshot{}
	err := r.db.GetReadPool().QueryRow(ctx, query, date.Format("2006-01-02")).Scan(
		&snapshot.ID,
		&snapshot.TotalVisits,
		&snapshot.DailyVisits,
		&snapshot.UniqueVisits,
		&snapshot.SnapshotDate,
		&snapshot.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get visitor snapshot by date: %w", err)
	}

	return snapshot, nil
}

// GetHistoricalSnapshots retrieves visitor snapshots within a date range
func (r *visitorRepository) GetHistoricalSnapshots(ctx context.Context, startDate, endDate time.Time, limit int) ([]*domain.VisitorSnapshot, error) {
	query := `
		SELECT id, total_visits, daily_visits, unique_visits, snapshot_date, created_at
		FROM visitor_snapshots
		WHERE snapshot_date >= $1 AND snapshot_date <= $2
		ORDER BY snapshot_date DESC
		LIMIT $3
	`

	rows, err := r.db.GetReadPool().Query(ctx, query,
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query historical visitor snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*domain.VisitorSnapshot
	for rows.Next() {
		snapshot := &domain.VisitorSnapshot{}
		err := rows.Scan(
			&snapshot.ID,
			&snapshot.TotalVisits,
			&snapshot.DailyVisits,
			&snapshot.UniqueVisits,
			&snapshot.SnapshotDate,
			&snapshot.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan visitor snapshot row: %w", err)
		}
		snapshots = append(snapshots, snapshot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading visitor snapshot rows: %w", err)
	}

	return snapshots, nil
}

// DeleteOldSnapshots removes snapshots older than the specified retention period
func (r *visitorRepository) DeleteOldSnapshots(ctx context.Context, retentionDays int) (int64, error) {
	query := `
		DELETE FROM visitor_snapshots
		WHERE snapshot_date < $1
	`

	cutoffDate := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02")
	result, err := r.db.Pool.Exec(ctx, query, cutoffDate)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old visitor snapshots: %w", err)
	}

	rowsAffected := result.RowsAffected()
	return rowsAffected, nil
}

// GetSnapshotCount returns the total number of snapshots in the database
func (r *visitorRepository) GetSnapshotCount(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM visitor_snapshots`

	var count int64
	err := r.db.GetReadPool().QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get visitor snapshot count: %w", err)
	}

	return count, nil
}