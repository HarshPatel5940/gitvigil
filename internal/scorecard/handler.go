package scorecard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/harshpatel5940/gitvigil/internal/database"
	"github.com/harshpatel5940/gitvigil/internal/models"
	"github.com/rs/zerolog"
)

type Handler struct {
	db     *database.DB
	logger zerolog.Logger
}

func NewHandler(db *database.DB, logger zerolog.Logger) *Handler {
	return &Handler{
		db:     db,
		logger: logger.With().Str("component", "scorecard").Logger(),
	}
}

type Scorecard struct {
	Repository      RepositoryInfo     `json:"repository"`
	OverallScore    int                `json:"overall_score"`
	OverallStatus   string             `json:"overall_status"`
	Checks          []CheckResult      `json:"checks"`
	Alerts          []AlertSummary     `json:"alerts"`
	Contributors    []ContributorStats `json:"contributors"`
	ActivitySummary ActivitySummary    `json:"activity_summary"`
	GeneratedAt     time.Time          `json:"generated_at"`
}

type RepositoryInfo struct {
	Owner      string `json:"owner"`
	Name       string `json:"name"`
	FullName   string `json:"full_name"`
	HasLicense bool   `json:"has_license"`
	LicenseID  string `json:"license_spdx_id,omitempty"`
}

type CheckResult struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Score       int    `json:"score"`
	Description string `json:"description"`
}

type AlertSummary struct {
	Type     string    `json:"type"`
	Severity string    `json:"severity"`
	Count    int       `json:"count"`
	LatestAt time.Time `json:"latest_at,omitempty"`
}

type ContributorStats struct {
	Login               string  `json:"login"`
	TotalCommits        int     `json:"total_commits"`
	Additions           int64   `json:"additions"`
	Deletions           int64   `json:"deletions"`
	CommitFrequency     float64 `json:"commit_frequency"`
	ContributionPattern string  `json:"contribution_pattern"`
}

type ActivitySummary struct {
	TotalCommits      int       `json:"total_commits"`
	LastActivityAt    time.Time `json:"last_activity_at"`
	StreakStatus      string    `json:"streak_status"`
	DaysSinceActivity int       `json:"days_since_activity"`
	ForcePushCount    int       `json:"force_push_count"`
	BackdateCount     int       `json:"backdate_count"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repoParam := r.URL.Query().Get("repo")
	if repoParam == "" {
		http.Error(w, "repo parameter is required (format: owner/name)", http.StatusBadRequest)
		return
	}

	parts := strings.SplitN(repoParam, "/", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid repo format, expected owner/name", http.StatusBadRequest)
		return
	}
	owner, name := parts[0], parts[1]

	ctx := r.Context()

	// Get repository
	repoStore := models.NewRepositoryStore(h.db.Pool)
	repo, err := repoStore.GetByFullName(ctx, owner, name)
	if err != nil {
		h.logger.Error().Err(err).Str("repo", repoParam).Msg("failed to get repository")
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	// Build scorecard
	scorecard, err := h.buildScorecard(ctx, repo)
	if err != nil {
		h.logger.Error().Err(err).Str("repo", repoParam).Msg("failed to build scorecard")
		http.Error(w, "failed to generate scorecard", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scorecard)
}

func (h *Handler) buildScorecard(ctx context.Context, repo *models.Repository) (*Scorecard, error) {
	commitStore := models.NewCommitStore(h.db.Pool)
	alertStore := models.NewAlertStore(h.db.Pool)
	contributorStore := models.NewContributorStore(h.db.Pool)

	// Get commit stats
	commitStats, err := commitStore.GetStats(ctx, repo.ID)
	if err != nil {
		return nil, err
	}

	// Get alert counts
	typeCounts, severityCounts, err := alertStore.CountByRepository(ctx, repo.ID)
	if err != nil {
		return nil, err
	}

	// Get contributors
	contributors, err := contributorStore.ListByRepository(ctx, repo.ID)
	if err != nil {
		return nil, err
	}

	// Build checks
	checks := h.buildChecks(repo, commitStats, typeCounts)

	// Calculate overall score
	overallScore := h.calculateOverallScore(checks)
	overallStatus := h.getOverallStatus(overallScore, severityCounts)

	// Build alert summaries
	alertSummaries := h.buildAlertSummaries(typeCounts)

	// Build contributor stats
	contributorStats := h.buildContributorStats(contributors, commitStats.TotalCommits)

	// Build activity summary
	daysSinceActivity := 0
	var lastActivityAt time.Time
	if repo.LastActivityAt != nil {
		lastActivityAt = *repo.LastActivityAt
		daysSinceActivity = int(time.Since(lastActivityAt).Hours() / 24)
	}

	forcePushCount := typeCounts[models.AlertForcePush]
	backdateCount := typeCounts[models.AlertBackdateSuspicious] + typeCounts[models.AlertBackdateCritical]

	licenseID := ""
	if repo.LicenseSPDXID != nil {
		licenseID = *repo.LicenseSPDXID
	}

	return &Scorecard{
		Repository: RepositoryInfo{
			Owner:      repo.Owner,
			Name:       repo.Name,
			FullName:   repo.FullName,
			HasLicense: repo.HasLicense,
			LicenseID:  licenseID,
		},
		OverallScore:  overallScore,
		OverallStatus: overallStatus,
		Checks:        checks,
		Alerts:        alertSummaries,
		Contributors:  contributorStats,
		ActivitySummary: ActivitySummary{
			TotalCommits:      commitStats.TotalCommits,
			LastActivityAt:    lastActivityAt,
			StreakStatus:      repo.StreakStatus,
			DaysSinceActivity: daysSinceActivity,
			ForcePushCount:    forcePushCount,
			BackdateCount:     backdateCount,
		},
		GeneratedAt: time.Now(),
	}, nil
}

func (h *Handler) buildChecks(repo *models.Repository, commitStats *models.CommitStats, alertCounts map[models.AlertType]int) []CheckResult {
	var checks []CheckResult

	// License check
	licenseScore := 0
	licenseStatus := "fail"
	licenseDesc := "No license file found"
	if repo.HasLicense {
		licenseScore = 100
		licenseStatus = "pass"
		if repo.LicenseSPDXID != nil {
			licenseDesc = "Repository has " + *repo.LicenseSPDXID + " license"
		} else {
			licenseDesc = "Repository has a license file"
		}
	}
	checks = append(checks, CheckResult{
		Name:        "License Present",
		Status:      licenseStatus,
		Score:       licenseScore,
		Description: licenseDesc,
	})

	// Backdate check
	backdateCount := alertCounts[models.AlertBackdateSuspicious] + alertCounts[models.AlertBackdateCritical]
	backdateScore := 100
	backdateStatus := "pass"
	backdateDesc := "No backdated commits detected"
	if backdateCount > 0 {
		backdateScore = max(0, 100-backdateCount*20)
		if backdateScore < 50 {
			backdateStatus = "fail"
		} else {
			backdateStatus = "warn"
		}
		backdateDesc = pluralize(backdateCount, "commit", "commits") + " with suspicious timestamps detected"
	}
	checks = append(checks, CheckResult{
		Name:        "No Backdated Commits",
		Status:      backdateStatus,
		Score:       backdateScore,
		Description: backdateDesc,
	})

	// Force push check
	forcePushCount := alertCounts[models.AlertForcePush]
	forcePushScore := 100
	forcePushStatus := "pass"
	forcePushDesc := "No force pushes detected"
	if forcePushCount > 0 {
		forcePushScore = max(0, 100-forcePushCount*25)
		if forcePushScore < 50 {
			forcePushStatus = "fail"
		} else {
			forcePushStatus = "warn"
		}
		forcePushDesc = pluralize(forcePushCount, "force push", "force pushes") + " detected"
	}
	checks = append(checks, CheckResult{
		Name:        "No Force Pushes",
		Status:      forcePushStatus,
		Score:       forcePushScore,
		Description: forcePushDesc,
	})

	// Streak check
	streakScore := 100
	streakStatus := "pass"
	streakDesc := "Repository has consistent activity"
	if repo.StreakStatus == "at_risk" {
		streakScore = 50
		streakStatus = "warn"
		streakDesc = "Repository activity streak is at risk"
	} else if repo.StreakStatus == "inactive" {
		streakScore = 0
		streakStatus = "fail"
		streakDesc = "Repository has been inactive"
	}
	checks = append(checks, CheckResult{
		Name:        "Activity Streak",
		Status:      streakStatus,
		Score:       streakScore,
		Description: streakDesc,
	})

	// Conventional commits check
	conventionalScore := 0
	conventionalStatus := "warn"
	conventionalDesc := "No conventional commits found"
	if commitStats.TotalCommits > 0 {
		conventionalPct := float64(commitStats.ConventionalCount) / float64(commitStats.TotalCommits) * 100
		conventionalScore = int(conventionalPct)
		if conventionalPct >= 80 {
			conventionalStatus = "pass"
		} else if conventionalPct >= 50 {
			conventionalStatus = "warn"
		} else {
			conventionalStatus = "fail"
		}
		conventionalDesc = pluralize(int(conventionalPct), "% of commits follow", "% of commits follow") + " conventional format"
	}
	checks = append(checks, CheckResult{
		Name:        "Conventional Commits",
		Status:      conventionalStatus,
		Score:       conventionalScore,
		Description: conventionalDesc,
	})

	return checks
}

func (h *Handler) calculateOverallScore(checks []CheckResult) int {
	if len(checks) == 0 {
		return 0
	}

	total := 0
	for _, check := range checks {
		total += check.Score
	}
	return total / len(checks)
}

func (h *Handler) getOverallStatus(score int, severityCounts map[models.Severity]int) string {
	if severityCounts[models.SeverityCritical] > 0 {
		return "critical"
	}
	if score >= 80 {
		return "healthy"
	}
	if score >= 50 {
		return "warning"
	}
	return "critical"
}

func (h *Handler) buildAlertSummaries(typeCounts map[models.AlertType]int) []AlertSummary {
	var summaries []AlertSummary

	severityMap := map[models.AlertType]string{
		models.AlertBackdateSuspicious: "warning",
		models.AlertBackdateCritical:   "critical",
		models.AlertForcePush:          "warning",
		models.AlertNoLicense:          "info",
		models.AlertStreakAtRisk:       "warning",
	}

	for alertType, count := range typeCounts {
		if count > 0 {
			summaries = append(summaries, AlertSummary{
				Type:     string(alertType),
				Severity: severityMap[alertType],
				Count:    count,
			})
		}
	}

	return summaries
}

func (h *Handler) buildContributorStats(contributors []*models.Contributor, totalCommits int) []ContributorStats {
	var stats []ContributorStats

	for _, c := range contributors {
		login := ""
		if c.GitHubLogin != nil {
			login = *c.GitHubLogin
		} else if c.Name != nil {
			login = *c.Name
		} else {
			login = c.Email
		}

		// Calculate commit frequency (commits per day)
		var frequency float64
		if c.FirstCommitAt != nil && c.LastCommitAt != nil {
			days := c.LastCommitAt.Sub(*c.FirstCommitAt).Hours() / 24
			if days > 0 {
				frequency = float64(c.TotalCommits) / days
			} else {
				frequency = float64(c.TotalCommits)
			}
		}

		// Determine contribution pattern
		pattern := "balanced"
		if totalCommits > 0 {
			pct := float64(c.TotalCommits) / float64(totalCommits) * 100
			if pct > 80 {
				pattern = "lone_wolf"
			}
		}

		stats = append(stats, ContributorStats{
			Login:               login,
			TotalCommits:        c.TotalCommits,
			Additions:           c.TotalAdditions,
			Deletions:           c.TotalDeletions,
			CommitFrequency:     frequency,
			ContributionPattern: pattern,
		})
	}

	return stats
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
