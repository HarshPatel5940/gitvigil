package detection

import (
	"context"
	"time"

	"github.com/harshpatel5940/gitvigil/internal/config"
	"github.com/harshpatel5940/gitvigil/internal/database"
	ghclient "github.com/harshpatel5940/gitvigil/internal/github"
	"github.com/harshpatel5940/gitvigil/internal/models"
	"github.com/rs/zerolog"
)

type Detector struct {
	cfg    *config.Config
	db     *database.DB
	gh     *ghclient.AppClient
	logger zerolog.Logger
}

func NewDetector(cfg *config.Config, db *database.DB, gh *ghclient.AppClient, logger zerolog.Logger) *Detector {
	return &Detector{
		cfg:    cfg,
		db:     db,
		gh:     gh,
		logger: logger.With().Str("component", "detector").Logger(),
	}
}

type BackdateResult struct {
	CommitSHA       string
	AuthorDate      time.Time
	PushedAt        time.Time
	DifferenceHours int
	IsSuspicious    bool
	IsCritical      bool
}

func (d *Detector) AnalyzeBackdate(authorDate, pushedAt time.Time) *BackdateResult {
	diffHours := int(pushedAt.Sub(authorDate).Hours())

	return &BackdateResult{
		AuthorDate:      authorDate,
		PushedAt:        pushedAt,
		DifferenceHours: diffHours,
		IsSuspicious:    diffHours > d.cfg.BackdateSuspiciousHours,
		IsCritical:      diffHours > d.cfg.BackdateCriticalHours,
	}
}

func (d *Detector) CheckLicense(ctx context.Context, installationID int64, owner, repo string) (bool, string, error) {
	client, err := d.gh.GetInstallationClient(installationID)
	if err != nil {
		return false, "", err
	}

	license, _, err := client.Repositories.License(ctx, owner, repo)
	if err != nil {
		// 404 means no license
		return false, "", nil
	}

	if license.License != nil {
		return true, license.License.GetSPDXID(), nil
	}

	return false, "", nil
}

func (d *Detector) CheckStreaks(ctx context.Context) error {
	repoStore := models.NewRepositoryStore(d.db.Pool)
	alertStore := models.NewAlertStore(d.db.Pool)

	repos, err := repoStore.ListAtRisk(ctx, d.cfg.StreakInactivityHours)
	if err != nil {
		return err
	}

	for _, repo := range repos {
		// Update streak status
		if err := repoStore.UpdateStreakStatus(ctx, repo.ID, "at_risk"); err != nil {
			d.logger.Error().Err(err).Int64("repo_id", repo.ID).Msg("failed to update streak status")
			continue
		}

		// Create alert
		alert := &models.Alert{
			RepositoryID: repo.ID,
			AlertType:    models.AlertStreakAtRisk,
			Severity:     models.SeverityWarning,
			Title:        "Activity streak at risk",
			Description:  "Repository has been inactive for more than 72 hours",
			Metadata: map[string]interface{}{
				"last_activity_at": repo.LastActivityAt,
				"inactivity_hours": d.cfg.StreakInactivityHours,
			},
		}

		if err := alertStore.Create(ctx, alert); err != nil {
			d.logger.Error().Err(err).Int64("repo_id", repo.ID).Msg("failed to create streak alert")
		}

		d.logger.Info().
			Str("repo", repo.FullName).
			Time("last_activity", *repo.LastActivityAt).
			Msg("repository marked as at risk")
	}

	return nil
}

func (d *Detector) ValidateLicenseForRepo(ctx context.Context, repoID, installationID int64, owner, name string) error {
	hasLicense, spdxID, err := d.CheckLicense(ctx, installationID, owner, name)
	if err != nil {
		return err
	}

	repoStore := models.NewRepositoryStore(d.db.Pool)
	var spdxPtr *string
	if spdxID != "" {
		spdxPtr = &spdxID
	}

	if err := repoStore.UpdateLicense(ctx, repoID, hasLicense, spdxPtr); err != nil {
		return err
	}

	// Create alert if no license
	if !hasLicense {
		alertStore := models.NewAlertStore(d.db.Pool)
		alert := &models.Alert{
			RepositoryID: repoID,
			AlertType:    models.AlertNoLicense,
			Severity:     models.SeverityInfo,
			Title:        "No license file found",
			Description:  "Repository does not have a LICENSE file",
		}
		if err := alertStore.Create(ctx, alert); err != nil {
			d.logger.Error().Err(err).Int64("repo_id", repoID).Msg("failed to create license alert")
		}
	}

	return nil
}
