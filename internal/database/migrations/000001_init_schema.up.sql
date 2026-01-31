-- Installations table: Track GitHub App installations
CREATE TABLE installations (
    id BIGSERIAL PRIMARY KEY,
    installation_id BIGINT UNIQUE NOT NULL,
    account_login VARCHAR(255) NOT NULL,
    account_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Repositories table: Monitored repositories
CREATE TABLE repositories (
    id BIGSERIAL PRIMARY KEY,
    github_id BIGINT UNIQUE NOT NULL,
    installation_id BIGINT REFERENCES installations(installation_id),
    owner VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    full_name VARCHAR(511) NOT NULL,
    default_branch VARCHAR(255) DEFAULT 'main',
    has_license BOOLEAN DEFAULT FALSE,
    license_spdx_id VARCHAR(50),
    last_push_at TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ,
    streak_status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(owner, name)
);

CREATE INDEX idx_repositories_installation ON repositories(installation_id);
CREATE INDEX idx_repositories_streak ON repositories(streak_status);

-- Contributors table: Track team members
CREATE TABLE contributors (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT REFERENCES repositories(id) ON DELETE CASCADE,
    github_login VARCHAR(255),
    email VARCHAR(255),
    name VARCHAR(255),
    total_commits INT DEFAULT 0,
    total_additions BIGINT DEFAULT 0,
    total_deletions BIGINT DEFAULT 0,
    first_commit_at TIMESTAMPTZ,
    last_commit_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(repository_id, email)
);

CREATE INDEX idx_contributors_repo ON contributors(repository_id);
