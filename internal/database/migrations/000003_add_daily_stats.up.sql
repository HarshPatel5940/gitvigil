-- Daily contribution stats for volume analysis
CREATE TABLE daily_stats (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT REFERENCES repositories(id) ON DELETE CASCADE,
    contributor_id BIGINT REFERENCES contributors(id) ON DELETE CASCADE,
    stat_date DATE NOT NULL,
    commit_count INT DEFAULT 0,
    additions INT DEFAULT 0,
    deletions INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(repository_id, contributor_id, stat_date)
);

CREATE INDEX idx_daily_stats_repo_date ON daily_stats(repository_id, stat_date);
CREATE INDEX idx_daily_stats_contributor ON daily_stats(contributor_id);
