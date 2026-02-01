package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/go-github/v68/github"
	"github.com/harshpatel5940/gitvigil/internal/config"
	"github.com/harshpatel5940/gitvigil/internal/database"
	ghclient "github.com/harshpatel5940/gitvigil/internal/github"
	"github.com/rs/zerolog"
)

type Handler struct {
	cfg    *config.Config
	db     *database.DB
	gh     *ghclient.AppClient
	logger zerolog.Logger
}

func NewHandler(cfg *config.Config, db *database.DB, gh *ghclient.AppClient, logger zerolog.Logger) *Handler {
	return &Handler{
		cfg:    cfg,
		db:     db,
		gh:     gh,
		logger: logger.With().Str("component", "webhook").Logger(),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if GitHub client is configured
	if h.gh == nil {
		h.logger.Error().Msg("webhook received but GitHub App not configured (missing private key)")
		http.Error(w, "GitHub App not configured", http.StatusServiceUnavailable)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to read request body")
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if h.cfg.WebhookSecret != "" {
		if err := ValidateSignature(body, signature, []byte(h.cfg.WebhookSecret)); err != nil {
			h.logger.Warn().
				Err(err).
				Str("signature_header", signature).
				Int("body_len", len(body)).
				Int("secret_len", len(h.cfg.WebhookSecret)).
				Str("secret_hex", fmt.Sprintf("%x", h.cfg.WebhookSecret)).
				Msg("signature validation failed")
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Get event type
	eventType := r.Header.Get("X-GitHub-Event")
	deliveryID := r.Header.Get("X-GitHub-Delivery")

	h.logger.Info().
		Str("event", eventType).
		Str("delivery_id", deliveryID).
		Msg("received webhook")

	// Record receive time for backdate detection
	receiveTime := time.Now()

	// Route event
	ctx := r.Context()
	switch eventType {
	case "push":
		h.handlePush(ctx, body, receiveTime)
	case "installation":
		h.handleInstallation(ctx, body)
	case "installation_repositories":
		h.handleInstallationRepositories(ctx, body)
	case "ping":
		h.logger.Info().Msg("received ping event")
	default:
		h.logger.Debug().Str("event", eventType).Msg("ignoring unhandled event type")
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handlePush(ctx context.Context, body []byte, receiveTime time.Time) {
	var event github.PushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Error().Err(err).Msg("failed to parse push event")
		return
	}

	repo := event.GetRepo()
	h.logger.Info().
		Str("repo", repo.GetFullName()).
		Str("ref", event.GetRef()).
		Int("commits", len(event.Commits)).
		Bool("forced", event.GetForced()).
		Str("pusher", event.GetPusher().GetLogin()).
		Msg("processing push event")

	// Store push event
	installationID := event.GetInstallation().GetID()
	if err := h.storePushEvent(ctx, &event, installationID, receiveTime); err != nil {
		h.logger.Error().Err(err).Msg("failed to store push event")
		return
	}

	// Process commits for backdate detection
	for _, commit := range event.Commits {
		if err := h.processCommit(ctx, repo, commit, installationID, receiveTime); err != nil {
			h.logger.Error().
				Err(err).
				Str("sha", commit.GetID()).
				Msg("failed to process commit")
		}
	}

	// Check for force push
	if event.GetForced() {
		h.createForcePushAlert(ctx, repo, &event)
	}
}

func (h *Handler) handleInstallation(ctx context.Context, body []byte) {
	var event github.InstallationEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Error().Err(err).Msg("failed to parse installation event")
		return
	}

	action := event.GetAction()
	installation := event.GetInstallation()
	account := installation.GetAccount()

	h.logger.Info().
		Str("action", action).
		Int64("installation_id", installation.GetID()).
		Str("account", account.GetLogin()).
		Msg("processing installation event")

	switch action {
	case "created":
		if err := h.storeInstallation(ctx, installation); err != nil {
			h.logger.Error().Err(err).Msg("failed to store installation")
		}
		// Store repositories
		for _, repo := range event.Repositories {
			if err := h.storeRepository(ctx, repo, installation.GetID()); err != nil {
				h.logger.Error().Err(err).Str("repo", repo.GetFullName()).Msg("failed to store repository")
			}
		}
	case "deleted":
		// Clean up installation data
		h.logger.Info().Int64("installation_id", installation.GetID()).Msg("installation deleted")
	}
}

func (h *Handler) handleInstallationRepositories(ctx context.Context, body []byte) {
	var event github.InstallationRepositoriesEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Error().Err(err).Msg("failed to parse installation_repositories event")
		return
	}

	installationID := event.GetInstallation().GetID()

	// Handle added repositories
	for _, repo := range event.RepositoriesAdded {
		if err := h.storeRepository(ctx, repo, installationID); err != nil {
			h.logger.Error().Err(err).Str("repo", repo.GetFullName()).Msg("failed to store added repository")
		}
	}

	// Handle removed repositories
	for _, repo := range event.RepositoriesRemoved {
		h.logger.Info().Str("repo", repo.GetFullName()).Msg("repository removed from installation")
	}
}

func (h *Handler) storePushEvent(ctx context.Context, event *github.PushEvent, installationID int64, receiveTime time.Time) error {
	repo := event.GetRepo()

	// First ensure repository exists
	_, err := h.db.Pool.Exec(ctx, `
		INSERT INTO repositories (github_id, installation_id, owner, name, full_name, last_push_at, last_activity_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (github_id) DO UPDATE SET
			last_push_at = $6,
			last_activity_at = $6,
			streak_status = 'active',
			updated_at = NOW()
	`, repo.GetID(), installationID, repo.GetOwner().GetLogin(), repo.GetName(), repo.GetFullName(), receiveTime)
	if err != nil {
		return err
	}

	// Get repository ID
	var repoID int64
	err = h.db.Pool.QueryRow(ctx, `SELECT id FROM repositories WHERE github_id = $1`, repo.GetID()).Scan(&repoID)
	if err != nil {
		return err
	}

	// Store push event
	_, err = h.db.Pool.Exec(ctx, `
		INSERT INTO push_events (repository_id, push_id, ref, before_sha, after_sha, forced, pusher_login, commit_count, distinct_count, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, repoID, event.GetPushID(), event.GetRef(), event.GetBefore(), event.GetAfter(),
		event.GetForced(), event.GetPusher().GetLogin(), len(event.Commits), event.GetDistinctSize(), receiveTime)

	return err
}

func (h *Handler) processCommit(ctx context.Context, repo *github.PushEventRepository, commit *github.HeadCommit, installationID int64, receiveTime time.Time) error {
	// Get repository ID
	var repoID int64
	err := h.db.Pool.QueryRow(ctx, `SELECT id FROM repositories WHERE github_id = $1`, repo.GetID()).Scan(&repoID)
	if err != nil {
		return err
	}

	// Get commit author date
	authorDate := commit.GetTimestamp().Time

	// Calculate backdate hours
	backdateHours := int(receiveTime.Sub(authorDate).Hours())
	isBackdated := backdateHours > h.cfg.BackdateSuspiciousHours

	// Determine conventional commit type
	isConventional, conventionalType, conventionalScope := parseConventionalCommit(commit.GetMessage())

	// Store commit
	_, err = h.db.Pool.Exec(ctx, `
		INSERT INTO commits (repository_id, sha, message, author_email, author_name, author_date, committer_date, pushed_at, additions, deletions, is_conventional, conventional_type, conventional_scope, is_backdated, backdate_hours)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (sha) DO NOTHING
	`, repoID, commit.GetID(), commit.GetMessage(),
		commit.GetAuthor().GetEmail(), commit.GetAuthor().GetName(),
		authorDate, commit.GetTimestamp().Time, receiveTime,
		0, 0, // additions/deletions not available in push event
		isConventional, conventionalType, conventionalScope,
		isBackdated, backdateHours)
	if err != nil {
		return err
	}

	// Create backdate alert if needed
	if isBackdated {
		severity := "warning"
		alertType := "backdate_suspicious"
		if backdateHours > h.cfg.BackdateCriticalHours {
			severity = "critical"
			alertType = "backdate_critical"
		}

		_, err = h.db.Pool.Exec(ctx, `
			INSERT INTO alerts (repository_id, commit_sha, alert_type, severity, title, description, metadata)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, repoID, commit.GetID(), alertType, severity,
			"Backdated commit detected",
			"Commit author date is significantly older than push time",
			map[string]interface{}{
				"author_date":    authorDate,
				"pushed_at":      receiveTime,
				"backdate_hours": backdateHours,
			})
		if err != nil {
			h.logger.Error().Err(err).Msg("failed to create backdate alert")
		}
	}

	// Update contributor stats
	if err := h.updateContributor(ctx, repoID, commit, receiveTime); err != nil {
		h.logger.Error().Err(err).Msg("failed to update contributor")
	}

	return nil
}

func (h *Handler) updateContributor(ctx context.Context, repoID int64, commit *github.HeadCommit, receiveTime time.Time) error {
	author := commit.GetAuthor()

	_, err := h.db.Pool.Exec(ctx, `
		INSERT INTO contributors (repository_id, github_login, email, name, total_commits, first_commit_at, last_commit_at)
		VALUES ($1, $2, $3, $4, 1, $5, $5)
		ON CONFLICT (repository_id, email) DO UPDATE SET
			github_login = COALESCE(EXCLUDED.github_login, contributors.github_login),
			name = COALESCE(EXCLUDED.name, contributors.name),
			total_commits = contributors.total_commits + 1,
			last_commit_at = $5,
			updated_at = NOW()
	`, repoID, author.GetLogin(), author.GetEmail(), author.GetName(), receiveTime)

	return err
}

func (h *Handler) createForcePushAlert(ctx context.Context, repo *github.PushEventRepository, event *github.PushEvent) {
	var repoID int64
	err := h.db.Pool.QueryRow(ctx, `SELECT id FROM repositories WHERE github_id = $1`, repo.GetID()).Scan(&repoID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get repository ID for force push alert")
		return
	}

	_, err = h.db.Pool.Exec(ctx, `
		INSERT INTO alerts (repository_id, alert_type, severity, title, description, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repoID, "force_push", "warning",
		"Force push detected",
		"Repository history was rewritten",
		map[string]interface{}{
			"ref":    event.GetRef(),
			"before": event.GetBefore(),
			"after":  event.GetAfter(),
			"pusher": event.GetPusher().GetLogin(),
		})
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to create force push alert")
	}
}

func (h *Handler) storeInstallation(ctx context.Context, installation *github.Installation) error {
	account := installation.GetAccount()
	_, err := h.db.Pool.Exec(ctx, `
		INSERT INTO installations (installation_id, account_login, account_type)
		VALUES ($1, $2, $3)
		ON CONFLICT (installation_id) DO UPDATE SET
			account_login = $2,
			updated_at = NOW()
	`, installation.GetID(), account.GetLogin(), account.GetType())
	return err
}

func (h *Handler) storeRepository(ctx context.Context, repo *github.Repository, installationID int64) error {
	_, err := h.db.Pool.Exec(ctx, `
		INSERT INTO repositories (github_id, installation_id, owner, name, full_name)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (github_id) DO UPDATE SET
			installation_id = $2,
			updated_at = NOW()
	`, repo.GetID(), installationID, repo.GetOwner().GetLogin(), repo.GetName(), repo.GetFullName())
	return err
}

func parseConventionalCommit(message string) (bool, string, string) {
	// Simple conventional commit parser
	// Format: type(scope): description or type: description
	if len(message) == 0 {
		return false, "", ""
	}

	// Find the colon
	colonIdx := -1
	for i, c := range message {
		if c == ':' {
			colonIdx = i
			break
		}
		if c == '\n' {
			break
		}
	}

	if colonIdx == -1 || colonIdx == 0 {
		return false, "", ""
	}

	prefix := message[:colonIdx]

	// Check for scope
	var commitType, scope string
	if parenStart := indexOf(prefix, '('); parenStart != -1 {
		if parenEnd := indexOf(prefix, ')'); parenEnd > parenStart {
			commitType = prefix[:parenStart]
			scope = prefix[parenStart+1 : parenEnd]
		} else {
			return false, "", ""
		}
	} else {
		commitType = prefix
	}

	// Validate type
	validTypes := map[string]bool{
		"feat": true, "fix": true, "docs": true, "style": true,
		"refactor": true, "perf": true, "test": true, "build": true,
		"ci": true, "chore": true, "revert": true,
	}

	if !validTypes[commitType] {
		return false, "", ""
	}

	return true, commitType, scope
}

func indexOf(s string, c rune) int {
	for i, r := range s {
		if r == c {
			return i
		}
	}
	return -1
}
