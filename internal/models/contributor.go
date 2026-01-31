package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Contributor struct {
	ID             int64
	RepositoryID   int64
	GitHubLogin    *string
	Email          string
	Name           *string
	TotalCommits   int
	TotalAdditions int64
	TotalDeletions int64
	FirstCommitAt  *time.Time
	LastCommitAt   *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ContributorStore struct {
	pool *pgxpool.Pool
}

func NewContributorStore(pool *pgxpool.Pool) *ContributorStore {
	return &ContributorStore{pool: pool}
}

func (s *ContributorStore) ListByRepository(ctx context.Context, repoID int64) ([]*Contributor, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, repository_id, github_login, email, name, total_commits,
		       total_additions, total_deletions, first_commit_at, last_commit_at,
		       created_at, updated_at
		FROM contributors WHERE repository_id = $1
		ORDER BY total_commits DESC
	`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contributors []*Contributor
	for rows.Next() {
		var c Contributor
		err := rows.Scan(
			&c.ID, &c.RepositoryID, &c.GitHubLogin, &c.Email, &c.Name,
			&c.TotalCommits, &c.TotalAdditions, &c.TotalDeletions,
			&c.FirstCommitAt, &c.LastCommitAt, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		contributors = append(contributors, &c)
	}
	return contributors, nil
}

func (s *ContributorStore) GetStats(ctx context.Context, repoID int64) (*ContributorStats, error) {
	var stats ContributorStats

	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) as total_contributors,
			SUM(total_commits) as total_commits,
			SUM(total_additions) as total_additions,
			SUM(total_deletions) as total_deletions
		FROM contributors WHERE repository_id = $1
	`, repoID).Scan(&stats.TotalContributors, &stats.TotalCommits, &stats.TotalAdditions, &stats.TotalDeletions)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

type ContributorStats struct {
	TotalContributors int
	TotalCommits      int
	TotalAdditions    int64
	TotalDeletions    int64
}

type DailyStat struct {
	ID            int64
	RepositoryID  int64
	ContributorID int64
	StatDate      time.Time
	CommitCount   int
	Additions     int
	Deletions     int
	CreatedAt     time.Time
}

type DailyStatsStore struct {
	pool *pgxpool.Pool
}

func NewDailyStatsStore(pool *pgxpool.Pool) *DailyStatsStore {
	return &DailyStatsStore{pool: pool}
}

func (s *DailyStatsStore) GetByRepository(ctx context.Context, repoID int64) ([]*DailyStat, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, repository_id, contributor_id, stat_date, commit_count, additions, deletions, created_at
		FROM daily_stats WHERE repository_id = $1
		ORDER BY stat_date DESC
	`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []*DailyStat
	for rows.Next() {
		var d DailyStat
		err := rows.Scan(
			&d.ID, &d.RepositoryID, &d.ContributorID, &d.StatDate,
			&d.CommitCount, &d.Additions, &d.Deletions, &d.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		stats = append(stats, &d)
	}
	return stats, nil
}

func (s *DailyStatsStore) Upsert(ctx context.Context, stat *DailyStat) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO daily_stats (repository_id, contributor_id, stat_date, commit_count, additions, deletions)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (repository_id, contributor_id, stat_date) DO UPDATE SET
			commit_count = daily_stats.commit_count + $4,
			additions = daily_stats.additions + $5,
			deletions = daily_stats.deletions + $6
	`, stat.RepositoryID, stat.ContributorID, stat.StatDate, stat.CommitCount, stat.Additions, stat.Deletions)
	return err
}
