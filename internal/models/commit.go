package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Commit struct {
	ID                int64
	RepositoryID      int64
	SHA               string
	Message           string
	AuthorEmail       string
	AuthorName        string
	AuthorDate        time.Time
	CommitterDate     time.Time
	PushedAt          time.Time
	Additions         int
	Deletions         int
	IsConventional    bool
	ConventionalType  *string
	ConventionalScope *string
	IsBackdated       bool
	BackdateHours     *int
	CreatedAt         time.Time
}

type CommitStore struct {
	pool *pgxpool.Pool
}

func NewCommitStore(pool *pgxpool.Pool) *CommitStore {
	return &CommitStore{pool: pool}
}

func (s *CommitStore) ListByRepository(ctx context.Context, repoID int64, limit int) ([]*Commit, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, repository_id, sha, message, author_email, author_name,
		       author_date, committer_date, pushed_at, additions, deletions,
		       is_conventional, conventional_type, conventional_scope,
		       is_backdated, backdate_hours, created_at
		FROM commits WHERE repository_id = $1
		ORDER BY pushed_at DESC
		LIMIT $2
	`, repoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commits []*Commit
	for rows.Next() {
		var c Commit
		err := rows.Scan(
			&c.ID, &c.RepositoryID, &c.SHA, &c.Message, &c.AuthorEmail, &c.AuthorName,
			&c.AuthorDate, &c.CommitterDate, &c.PushedAt, &c.Additions, &c.Deletions,
			&c.IsConventional, &c.ConventionalType, &c.ConventionalScope,
			&c.IsBackdated, &c.BackdateHours, &c.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		commits = append(commits, &c)
	}
	return commits, nil
}

func (s *CommitStore) GetStats(ctx context.Context, repoID int64) (*CommitStats, error) {
	var stats CommitStats

	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) as total_commits,
			COUNT(*) FILTER (WHERE is_backdated) as backdated_count,
			COUNT(*) FILTER (WHERE is_conventional) as conventional_count,
			SUM(additions) as total_additions,
			SUM(deletions) as total_deletions
		FROM commits WHERE repository_id = $1
	`, repoID).Scan(
		&stats.TotalCommits, &stats.BackdatedCount, &stats.ConventionalCount,
		&stats.TotalAdditions, &stats.TotalDeletions,
	)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

type CommitStats struct {
	TotalCommits      int
	BackdatedCount    int
	ConventionalCount int
	TotalAdditions    int64
	TotalDeletions    int64
}

func (s *CommitStore) CountBackdated(ctx context.Context, repoID int64) (suspicious, critical int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE is_backdated AND backdate_hours <= 72) as suspicious,
			COUNT(*) FILTER (WHERE backdate_hours > 72) as critical
		FROM commits WHERE repository_id = $1 AND is_backdated = TRUE
	`, repoID).Scan(&suspicious, &critical)
	return
}
