package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Installation struct {
	ID             int64
	InstallationID int64
	AccountLogin   string
	AccountType    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type InstallationWithStats struct {
	Installation
	RepoCount   int
	AlertCount  int
	CommitCount int
}

type InstallationStore struct {
	pool *pgxpool.Pool
}

func NewInstallationStore(pool *pgxpool.Pool) *InstallationStore {
	return &InstallationStore{pool: pool}
}

func (s *InstallationStore) List(ctx context.Context) ([]*InstallationWithStats, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			i.id, i.installation_id, i.account_login, i.account_type, i.created_at, i.updated_at,
			COALESCE(r.repo_count, 0) as repo_count,
			COALESCE(a.alert_count, 0) as alert_count,
			COALESCE(c.commit_count, 0) as commit_count
		FROM installations i
		LEFT JOIN (
			SELECT installation_id, COUNT(*) as repo_count
			FROM repositories
			GROUP BY installation_id
		) r ON r.installation_id = i.installation_id
		LEFT JOIN (
			SELECT r.installation_id, COUNT(*) as alert_count
			FROM alerts al
			JOIN repositories r ON r.id = al.repository_id
			GROUP BY r.installation_id
		) a ON a.installation_id = i.installation_id
		LEFT JOIN (
			SELECT r.installation_id, COUNT(*) as commit_count
			FROM commits c
			JOIN repositories r ON r.id = c.repository_id
			GROUP BY r.installation_id
		) c ON c.installation_id = i.installation_id
		ORDER BY i.account_login
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var installations []*InstallationWithStats
	for rows.Next() {
		var i InstallationWithStats
		err := rows.Scan(
			&i.ID, &i.InstallationID, &i.AccountLogin, &i.AccountType,
			&i.CreatedAt, &i.UpdatedAt,
			&i.RepoCount, &i.AlertCount, &i.CommitCount,
		)
		if err != nil {
			return nil, err
		}
		installations = append(installations, &i)
	}
	return installations, nil
}

func (s *InstallationStore) GetByID(ctx context.Context, installationID int64) (*InstallationWithStats, error) {
	var i InstallationWithStats
	err := s.pool.QueryRow(ctx, `
		SELECT
			i.id, i.installation_id, i.account_login, i.account_type, i.created_at, i.updated_at,
			COALESCE(r.repo_count, 0) as repo_count,
			COALESCE(a.alert_count, 0) as alert_count,
			COALESCE(c.commit_count, 0) as commit_count
		FROM installations i
		LEFT JOIN (
			SELECT installation_id, COUNT(*) as repo_count
			FROM repositories
			GROUP BY installation_id
		) r ON r.installation_id = i.installation_id
		LEFT JOIN (
			SELECT r.installation_id, COUNT(*) as alert_count
			FROM alerts al
			JOIN repositories r ON r.id = al.repository_id
			GROUP BY r.installation_id
		) a ON a.installation_id = i.installation_id
		LEFT JOIN (
			SELECT r.installation_id, COUNT(*) as commit_count
			FROM commits c
			JOIN repositories r ON r.id = c.repository_id
			GROUP BY r.installation_id
		) c ON c.installation_id = i.installation_id
		WHERE i.installation_id = $1
	`, installationID).Scan(
		&i.ID, &i.InstallationID, &i.AccountLogin, &i.AccountType,
		&i.CreatedAt, &i.UpdatedAt,
		&i.RepoCount, &i.AlertCount, &i.CommitCount,
	)
	if err != nil {
		return nil, err
	}
	return &i, nil
}
