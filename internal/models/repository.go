package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	ID             int64
	GitHubID       int64
	InstallationID int64
	Owner          string
	Name           string
	FullName       string
	DefaultBranch  string
	HasLicense     bool
	LicenseSPDXID  *string
	LastPushAt     *time.Time
	LastActivityAt *time.Time
	StreakStatus   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type RepositoryWithStats struct {
	Repository
	AlertsCount  int
	CommitsCount int
}

type RepositoryStore struct {
	pool *pgxpool.Pool
}

func NewRepositoryStore(pool *pgxpool.Pool) *RepositoryStore {
	return &RepositoryStore{pool: pool}
}

func (s *RepositoryStore) GetByGitHubID(ctx context.Context, githubID int64) (*Repository, error) {
	var r Repository
	err := s.pool.QueryRow(ctx, `
		SELECT id, github_id, installation_id, owner, name, full_name, default_branch,
		       has_license, license_spdx_id, last_push_at, last_activity_at, streak_status,
		       created_at, updated_at
		FROM repositories WHERE github_id = $1
	`, githubID).Scan(
		&r.ID, &r.GitHubID, &r.InstallationID, &r.Owner, &r.Name, &r.FullName,
		&r.DefaultBranch, &r.HasLicense, &r.LicenseSPDXID, &r.LastPushAt,
		&r.LastActivityAt, &r.StreakStatus, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *RepositoryStore) GetByFullName(ctx context.Context, owner, name string) (*Repository, error) {
	var r Repository
	err := s.pool.QueryRow(ctx, `
		SELECT id, github_id, installation_id, owner, name, full_name, default_branch,
		       has_license, license_spdx_id, last_push_at, last_activity_at, streak_status,
		       created_at, updated_at
		FROM repositories WHERE owner = $1 AND name = $2
	`, owner, name).Scan(
		&r.ID, &r.GitHubID, &r.InstallationID, &r.Owner, &r.Name, &r.FullName,
		&r.DefaultBranch, &r.HasLicense, &r.LicenseSPDXID, &r.LastPushAt,
		&r.LastActivityAt, &r.StreakStatus, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *RepositoryStore) UpdateLicense(ctx context.Context, id int64, hasLicense bool, spdxID *string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE repositories SET has_license = $2, license_spdx_id = $3, updated_at = NOW()
		WHERE id = $1
	`, id, hasLicense, spdxID)
	return err
}

func (s *RepositoryStore) UpdateStreakStatus(ctx context.Context, id int64, status string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE repositories SET streak_status = $2, updated_at = NOW()
		WHERE id = $1
	`, id, status)
	return err
}

func (s *RepositoryStore) ListByInstallation(ctx context.Context, installationID int64) ([]*Repository, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, github_id, installation_id, owner, name, full_name, default_branch,
		       has_license, license_spdx_id, last_push_at, last_activity_at, streak_status,
		       created_at, updated_at
		FROM repositories WHERE installation_id = $1
		ORDER BY full_name
	`, installationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []*Repository
	for rows.Next() {
		var r Repository
		err := rows.Scan(
			&r.ID, &r.GitHubID, &r.InstallationID, &r.Owner, &r.Name, &r.FullName,
			&r.DefaultBranch, &r.HasLicense, &r.LicenseSPDXID, &r.LastPushAt,
			&r.LastActivityAt, &r.StreakStatus, &r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		repos = append(repos, &r)
	}
	return repos, nil
}

func (s *RepositoryStore) ListAll(ctx context.Context, limit, offset int) ([]*RepositoryWithStats, int, error) {
	// Get total count
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM repositories`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			r.id, r.github_id, r.installation_id, r.owner, r.name, r.full_name, r.default_branch,
			r.has_license, r.license_spdx_id, r.last_push_at, r.last_activity_at, r.streak_status,
			r.created_at, r.updated_at,
			COALESCE(a.alert_count, 0) as alerts_count,
			COALESCE(c.commit_count, 0) as commits_count
		FROM repositories r
		LEFT JOIN (
			SELECT repository_id, COUNT(*) as alert_count
			FROM alerts
			GROUP BY repository_id
		) a ON a.repository_id = r.id
		LEFT JOIN (
			SELECT repository_id, COUNT(*) as commit_count
			FROM commits
			GROUP BY repository_id
		) c ON c.repository_id = r.id
		ORDER BY r.full_name
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var repos []*RepositoryWithStats
	for rows.Next() {
		var r RepositoryWithStats
		err := rows.Scan(
			&r.ID, &r.GitHubID, &r.InstallationID, &r.Owner, &r.Name, &r.FullName,
			&r.DefaultBranch, &r.HasLicense, &r.LicenseSPDXID, &r.LastPushAt,
			&r.LastActivityAt, &r.StreakStatus, &r.CreatedAt, &r.UpdatedAt,
			&r.AlertsCount, &r.CommitsCount,
		)
		if err != nil {
			return nil, 0, err
		}
		repos = append(repos, &r)
	}
	return repos, total, nil
}

func (s *RepositoryStore) GetByID(ctx context.Context, id int64) (*RepositoryWithStats, error) {
	var r RepositoryWithStats
	err := s.pool.QueryRow(ctx, `
		SELECT
			r.id, r.github_id, r.installation_id, r.owner, r.name, r.full_name, r.default_branch,
			r.has_license, r.license_spdx_id, r.last_push_at, r.last_activity_at, r.streak_status,
			r.created_at, r.updated_at,
			COALESCE(a.alert_count, 0) as alerts_count,
			COALESCE(c.commit_count, 0) as commits_count
		FROM repositories r
		LEFT JOIN (
			SELECT repository_id, COUNT(*) as alert_count
			FROM alerts
			GROUP BY repository_id
		) a ON a.repository_id = r.id
		LEFT JOIN (
			SELECT repository_id, COUNT(*) as commit_count
			FROM commits
			GROUP BY repository_id
		) c ON c.repository_id = r.id
		WHERE r.id = $1
	`, id).Scan(
		&r.ID, &r.GitHubID, &r.InstallationID, &r.Owner, &r.Name, &r.FullName,
		&r.DefaultBranch, &r.HasLicense, &r.LicenseSPDXID, &r.LastPushAt,
		&r.LastActivityAt, &r.StreakStatus, &r.CreatedAt, &r.UpdatedAt,
		&r.AlertsCount, &r.CommitsCount,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *RepositoryStore) ListAtRisk(ctx context.Context, inactivityHours int) ([]*Repository, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, github_id, installation_id, owner, name, full_name, default_branch,
		       has_license, license_spdx_id, last_push_at, last_activity_at, streak_status,
		       created_at, updated_at
		FROM repositories
		WHERE last_activity_at < NOW() - INTERVAL '1 hour' * $1
		  AND streak_status = 'active'
		ORDER BY last_activity_at
	`, inactivityHours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []*Repository
	for rows.Next() {
		var r Repository
		err := rows.Scan(
			&r.ID, &r.GitHubID, &r.InstallationID, &r.Owner, &r.Name, &r.FullName,
			&r.DefaultBranch, &r.HasLicense, &r.LicenseSPDXID, &r.LastPushAt,
			&r.LastActivityAt, &r.StreakStatus, &r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		repos = append(repos, &r)
	}
	return repos, nil
}
