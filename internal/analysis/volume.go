package analysis

import (
	"sort"
	"time"
)

// DailyActivity represents activity on a single day
type DailyActivity struct {
	Date      time.Time
	Commits   int
	Additions int
	Deletions int
}

// VolumeAnalysis contains the analysis of code volume patterns
type VolumeAnalysis struct {
	TotalDays         int            `json:"total_days"`
	ActiveDays        int            `json:"active_days"`
	TotalCommits      int            `json:"total_commits"`
	TotalAdditions    int            `json:"total_additions"`
	TotalDeletions    int            `json:"total_deletions"`
	AveragePerDay     float64        `json:"average_commits_per_day"`
	MaxDailyCommits   int            `json:"max_daily_commits"`
	Pattern           string         `json:"pattern"`
	PatternDesc       string         `json:"pattern_description"`
	ConsistencyScore  float64        `json:"consistency_score"`
	LastDayPercentage float64        `json:"last_day_percentage"`
	DailyBreakdown    []DayBreakdown `json:"daily_breakdown,omitempty"`
}

// DayBreakdown represents a single day's activity
type DayBreakdown struct {
	Date      string  `json:"date"`
	Commits   int     `json:"commits"`
	Additions int     `json:"additions"`
	Deletions int     `json:"deletions"`
	Pct       float64 `json:"percentage"`
}

// AnalyzeVolume analyzes the volume and timing patterns of contributions
func AnalyzeVolume(activities []DailyActivity, hackathonStart, hackathonEnd time.Time) *VolumeAnalysis {
	analysis := &VolumeAnalysis{
		DailyBreakdown: make([]DayBreakdown, 0),
	}

	if len(activities) == 0 {
		analysis.Pattern = "no_activity"
		analysis.PatternDesc = "No activity recorded"
		return analysis
	}

	// Sort by date
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Date.Before(activities[j].Date)
	})

	// Calculate totals
	for _, a := range activities {
		if a.Commits > 0 {
			analysis.ActiveDays++
		}
		analysis.TotalCommits += a.Commits
		analysis.TotalAdditions += a.Additions
		analysis.TotalDeletions += a.Deletions

		if a.Commits > analysis.MaxDailyCommits {
			analysis.MaxDailyCommits = a.Commits
		}
	}

	// Calculate total days in period
	if !hackathonStart.IsZero() && !hackathonEnd.IsZero() {
		analysis.TotalDays = int(hackathonEnd.Sub(hackathonStart).Hours()/24) + 1
	} else if len(activities) > 0 {
		analysis.TotalDays = int(activities[len(activities)-1].Date.Sub(activities[0].Date).Hours()/24) + 1
	}

	// Calculate average
	if analysis.TotalDays > 0 {
		analysis.AveragePerDay = float64(analysis.TotalCommits) / float64(analysis.TotalDays)
	}

	// Calculate daily breakdown and percentages
	for _, a := range activities {
		var pct float64
		if analysis.TotalCommits > 0 {
			pct = float64(a.Commits) / float64(analysis.TotalCommits) * 100
		}

		analysis.DailyBreakdown = append(analysis.DailyBreakdown, DayBreakdown{
			Date:      a.Date.Format("2006-01-02"),
			Commits:   a.Commits,
			Additions: a.Additions,
			Deletions: a.Deletions,
			Pct:       pct,
		})
	}

	// Calculate last day percentage
	if len(activities) > 0 && analysis.TotalCommits > 0 {
		lastDay := activities[len(activities)-1]
		analysis.LastDayPercentage = float64(lastDay.Commits) / float64(analysis.TotalCommits) * 100
	}

	// Calculate consistency score
	analysis.ConsistencyScore = calculateConsistencyScore(activities, analysis.TotalDays)

	// Determine pattern
	analysis.determinePattern()

	return analysis
}

func (a *VolumeAnalysis) determinePattern() {
	if a.TotalCommits == 0 {
		a.Pattern = "no_activity"
		a.PatternDesc = "No activity recorded"
		return
	}

	// Check for deadline dumper: >50% of commits in last 20% of time
	lastPeriodPct := 20.0
	if a.TotalDays > 0 {
		lastDays := int(float64(a.TotalDays) * lastPeriodPct / 100)
		if lastDays < 1 {
			lastDays = 1
		}

		// Count commits in last period
		lastPeriodCommits := 0
		if len(a.DailyBreakdown) > 0 {
			startIdx := len(a.DailyBreakdown) - lastDays
			if startIdx < 0 {
				startIdx = 0
			}
			for i := startIdx; i < len(a.DailyBreakdown); i++ {
				lastPeriodCommits += a.DailyBreakdown[i].Commits
			}
		}

		lastPeriodPctActual := float64(lastPeriodCommits) / float64(a.TotalCommits) * 100
		if lastPeriodPctActual > 50 {
			a.Pattern = "deadline_dumper"
			a.PatternDesc = "Most commits pushed near the deadline (>50% in final 20% of time)"
			return
		}
	}

	// Check for daily builder: consistent activity
	if a.ConsistencyScore >= 70 {
		a.Pattern = "daily_builder"
		a.PatternDesc = "Consistent daily activity throughout the period"
		return
	}

	if a.ConsistencyScore >= 40 {
		a.Pattern = "moderate_builder"
		a.PatternDesc = "Moderately consistent activity"
		return
	}

	// Check for burst pattern
	if float64(a.MaxDailyCommits) > a.AveragePerDay*3 {
		a.Pattern = "burst_coder"
		a.PatternDesc = "Activity comes in bursts with quiet periods between"
		return
	}

	a.Pattern = "sporadic"
	a.PatternDesc = "Irregular activity pattern"
}

// calculateConsistencyScore calculates how consistent the activity is
// Returns 0-100, where 100 is perfectly consistent
func calculateConsistencyScore(activities []DailyActivity, totalDays int) float64 {
	if totalDays == 0 || len(activities) == 0 {
		return 0
	}

	// Calculate activity rate
	activeDays := 0
	for _, a := range activities {
		if a.Commits > 0 {
			activeDays++
		}
	}

	activityRate := float64(activeDays) / float64(totalDays) * 100

	// Calculate variance in daily commits
	var sum float64
	for _, a := range activities {
		sum += float64(a.Commits)
	}
	mean := sum / float64(len(activities))

	var variance float64
	for _, a := range activities {
		diff := float64(a.Commits) - mean
		variance += diff * diff
	}
	variance /= float64(len(activities))

	// Lower variance = more consistent
	// Normalize variance to a 0-100 scale (inverse)
	varianceScore := 100 / (1 + variance/mean)

	// Combine activity rate and variance score
	return (activityRate + varianceScore) / 2
}

// ContributorVolumePattern represents a contributor's work pattern
type ContributorVolumePattern struct {
	Login          string  `json:"login"`
	Pattern        string  `json:"pattern"`
	TotalCommits   int     `json:"total_commits"`
	DailyAverage   float64 `json:"daily_average"`
	PeakDay        string  `json:"peak_day"`
	PeakDayCommits int     `json:"peak_day_commits"`
}

// AnalyzeContributorPatterns analyzes each contributor's work pattern
func AnalyzeContributorPatterns(contributorActivities map[string][]DailyActivity, hackathonStart, hackathonEnd time.Time) []ContributorVolumePattern {
	var patterns []ContributorVolumePattern

	for login, activities := range contributorActivities {
		analysis := AnalyzeVolume(activities, hackathonStart, hackathonEnd)

		peakDay := ""
		peakCommits := 0
		for _, a := range activities {
			if a.Commits > peakCommits {
				peakCommits = a.Commits
				peakDay = a.Date.Format("2006-01-02")
			}
		}

		patterns = append(patterns, ContributorVolumePattern{
			Login:          login,
			Pattern:        analysis.Pattern,
			TotalCommits:   analysis.TotalCommits,
			DailyAverage:   analysis.AveragePerDay,
			PeakDay:        peakDay,
			PeakDayCommits: peakCommits,
		})
	}

	// Sort by total commits
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].TotalCommits > patterns[j].TotalCommits
	})

	return patterns
}
