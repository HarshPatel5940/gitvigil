-- Commits table: Store commit metadata for analysis
CREATE TABLE commits (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT REFERENCES repositories(id) ON DELETE CASCADE,
    sha VARCHAR(40) UNIQUE NOT NULL,
    message TEXT,
    author_email VARCHAR(255),
    author_name VARCHAR(255),
    author_date TIMESTAMPTZ NOT NULL,
    committer_date TIMESTAMPTZ NOT NULL,
    pushed_at TIMESTAMPTZ NOT NULL,
    additions INT DEFAULT 0,
    deletions INT DEFAULT 0,
    is_conventional BOOLEAN DEFAULT FALSE,
    conventional_type VARCHAR(50),
    conventional_scope VARCHAR(100),
    is_backdated BOOLEAN DEFAULT FALSE,
    backdate_hours INT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_commits_repo ON commits(repository_id);
CREATE INDEX idx_commits_pushed_at ON commits(pushed_at);
CREATE INDEX idx_commits_backdated ON commits(is_backdated) WHERE is_backdated = TRUE;

-- Push events table: Track all push events
CREATE TABLE push_events (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT REFERENCES repositories(id) ON DELETE CASCADE,
    push_id BIGINT,
    ref VARCHAR(255),
    before_sha VARCHAR(40),
    after_sha VARCHAR(40),
    forced BOOLEAN DEFAULT FALSE,
    pusher_login VARCHAR(255),
    commit_count INT,
    distinct_count INT,
    received_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_push_events_repo ON push_events(repository_id);
CREATE INDEX idx_push_events_forced ON push_events(forced) WHERE forced = TRUE;

-- Alerts table: Detection alerts and flags
CREATE TABLE alerts (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT REFERENCES repositories(id) ON DELETE CASCADE,
    commit_sha VARCHAR(40),
    push_event_id BIGINT REFERENCES push_events(id),
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    metadata JSONB,
    acknowledged BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_alerts_repo ON alerts(repository_id);
CREATE INDEX idx_alerts_type ON alerts(alert_type);
CREATE INDEX idx_alerts_severity ON alerts(severity);
CREATE INDEX idx_alerts_unacked ON alerts(acknowledged) WHERE acknowledged = FALSE;
