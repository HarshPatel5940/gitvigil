package api

import (
	"net/http"
	"time"
)

type StatsResponse struct {
	Installations    int            `json:"installations"`
	Repositories     int            `json:"repositories"`
	TotalCommits     int            `json:"total_commits"`
	TotalAlerts      int            `json:"total_alerts"`
	ActiveRepos      int            `json:"active_repos"`
	AtRiskRepos      int            `json:"at_risk_repos"`
	BackdateAlerts   int            `json:"backdate_alerts"`
	ForcePushAlerts  int            `json:"force_push_alerts"`
	AlertsBySeverity map[string]int `json:"alerts_by_severity"`
	GeneratedAt      time.Time      `json:"generated_at"`
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats := StatsResponse{
		AlertsBySeverity: make(map[string]int),
		GeneratedAt:      time.Now(),
	}

	// Get counts
	queries := []struct {
		query  string
		target *int
	}{
		{"SELECT COUNT(*) FROM installations", &stats.Installations},
		{"SELECT COUNT(*) FROM repositories", &stats.Repositories},
		{"SELECT COUNT(*) FROM commits", &stats.TotalCommits},
		{"SELECT COUNT(*) FROM alerts", &stats.TotalAlerts},
		{"SELECT COUNT(*) FROM repositories WHERE streak_status = 'active'", &stats.ActiveRepos},
		{"SELECT COUNT(*) FROM repositories WHERE streak_status = 'at_risk'", &stats.AtRiskRepos},
		{"SELECT COUNT(*) FROM alerts WHERE alert_type LIKE 'backdate%'", &stats.BackdateAlerts},
		{"SELECT COUNT(*) FROM alerts WHERE alert_type = 'force_push'", &stats.ForcePushAlerts},
	}

	for _, q := range queries {
		if err := h.db.Pool.QueryRow(ctx, q.query).Scan(q.target); err != nil {
			h.logger.Error().Err(err).Str("query", q.query).Msg("failed to get stat")
			// Continue with zero value
		}
	}

	// Get alerts by severity
	rows, err := h.db.Pool.Query(ctx, `
		SELECT severity, COUNT(*) as count
		FROM alerts
		GROUP BY severity
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var severity string
			var count int
			if err := rows.Scan(&severity, &count); err == nil {
				stats.AlertsBySeverity[severity] = count
			}
		}
	}

	h.respondJSON(w, http.StatusOK, stats)
}
