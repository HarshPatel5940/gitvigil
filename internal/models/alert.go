package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AlertType string

const (
	AlertBackdateSuspicious AlertType = "backdate_suspicious"
	AlertBackdateCritical   AlertType = "backdate_critical"
	AlertForcePush          AlertType = "force_push"
	AlertNoLicense          AlertType = "no_license"
	AlertStreakAtRisk       AlertType = "streak_at_risk"
	AlertNonConventional    AlertType = "non_conventional_commit"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Alert struct {
	ID           int64
	RepositoryID int64
	CommitSHA    *string
	PushEventID  *int64
	AlertType    AlertType
	Severity     Severity
	Title        string
	Description  string
	Metadata     map[string]interface{}
	Acknowledged bool
	CreatedAt    time.Time
}

type AlertStore struct {
	pool *pgxpool.Pool
}

func NewAlertStore(pool *pgxpool.Pool) *AlertStore {
	return &AlertStore{pool: pool}
}

func (s *AlertStore) Create(ctx context.Context, alert *Alert) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO alerts (repository_id, commit_sha, push_event_id, alert_type, severity, title, description, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`, alert.RepositoryID, alert.CommitSHA, alert.PushEventID, alert.AlertType,
		alert.Severity, alert.Title, alert.Description, alert.Metadata,
	).Scan(&alert.ID, &alert.CreatedAt)
}

func (s *AlertStore) ListByRepository(ctx context.Context, repoID int64) ([]*Alert, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, repository_id, commit_sha, push_event_id, alert_type, severity,
		       title, description, metadata, acknowledged, created_at
		FROM alerts WHERE repository_id = $1
		ORDER BY created_at DESC
	`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*Alert
	for rows.Next() {
		var a Alert
		err := rows.Scan(
			&a.ID, &a.RepositoryID, &a.CommitSHA, &a.PushEventID, &a.AlertType,
			&a.Severity, &a.Title, &a.Description, &a.Metadata, &a.Acknowledged, &a.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, &a)
	}
	return alerts, nil
}

func (s *AlertStore) CountByRepository(ctx context.Context, repoID int64) (map[AlertType]int, map[Severity]int, error) {
	typeCounts := make(map[AlertType]int)
	severityCounts := make(map[Severity]int)

	rows, err := s.pool.Query(ctx, `
		SELECT alert_type, severity, COUNT(*) as count
		FROM alerts WHERE repository_id = $1
		GROUP BY alert_type, severity
	`, repoID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var alertType AlertType
		var severity Severity
		var count int
		if err := rows.Scan(&alertType, &severity, &count); err != nil {
			return nil, nil, err
		}
		typeCounts[alertType] += count
		severityCounts[severity] += count
	}

	return typeCounts, severityCounts, nil
}

func (s *AlertStore) Acknowledge(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE alerts SET acknowledged = TRUE WHERE id = $1
	`, id)
	return err
}
