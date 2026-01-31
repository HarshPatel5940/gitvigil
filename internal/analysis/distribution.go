package analysis

import (
	"math"
	"sort"
)

// ContributorData represents a contributor's activity data
type ContributorData struct {
	Login     string
	Commits   int
	Additions int64
	Deletions int64
}

// DistributionAnalysis contains the analysis of contribution distribution
type DistributionAnalysis struct {
	TotalContributors int                `json:"total_contributors"`
	TotalCommits      int                `json:"total_commits"`
	GiniCoefficient   float64            `json:"gini_coefficient"`
	TopContributor    *ContributorShare  `json:"top_contributor"`
	Contributors      []ContributorShare `json:"contributors"`
	Pattern           string             `json:"pattern"`
	PatternDesc       string             `json:"pattern_description"`
}

// ContributorShare represents a contributor's share of the work
type ContributorShare struct {
	Login       string  `json:"login"`
	Commits     int     `json:"commits"`
	CommitShare float64 `json:"commit_share"`
	Additions   int64   `json:"additions"`
	Deletions   int64   `json:"deletions"`
	CodeShare   float64 `json:"code_share"`
}

// AnalyzeDistribution analyzes the distribution of contributions
func AnalyzeDistribution(contributors []ContributorData) *DistributionAnalysis {
	analysis := &DistributionAnalysis{
		TotalContributors: len(contributors),
		Contributors:      make([]ContributorShare, 0, len(contributors)),
	}

	if len(contributors) == 0 {
		analysis.Pattern = "no_activity"
		analysis.PatternDesc = "No contributions found"
		return analysis
	}

	// Calculate totals
	var totalCommits int
	var totalLines int64
	for _, c := range contributors {
		totalCommits += c.Commits
		totalLines += c.Additions + c.Deletions
	}
	analysis.TotalCommits = totalCommits

	// Calculate shares
	commitShares := make([]float64, len(contributors))
	for i, c := range contributors {
		var commitShare, codeShare float64
		if totalCommits > 0 {
			commitShare = float64(c.Commits) / float64(totalCommits) * 100
		}
		if totalLines > 0 {
			codeShare = float64(c.Additions+c.Deletions) / float64(totalLines) * 100
		}

		commitShares[i] = commitShare

		share := ContributorShare{
			Login:       c.Login,
			Commits:     c.Commits,
			CommitShare: commitShare,
			Additions:   c.Additions,
			Deletions:   c.Deletions,
			CodeShare:   codeShare,
		}
		analysis.Contributors = append(analysis.Contributors, share)
	}

	// Sort by commits descending
	sort.Slice(analysis.Contributors, func(i, j int) bool {
		return analysis.Contributors[i].Commits > analysis.Contributors[j].Commits
	})

	if len(analysis.Contributors) > 0 {
		analysis.TopContributor = &analysis.Contributors[0]
	}

	// Calculate Gini coefficient for commit distribution
	analysis.GiniCoefficient = calculateGini(commitShares)

	// Determine pattern
	analysis.determinePattern()

	return analysis
}

func (a *DistributionAnalysis) determinePattern() {
	if a.TotalContributors == 0 {
		a.Pattern = "no_activity"
		a.PatternDesc = "No contributions found"
		return
	}

	if a.TotalContributors == 1 {
		a.Pattern = "solo"
		a.PatternDesc = "Single contributor project"
		return
	}

	if a.TopContributor != nil && a.TopContributor.CommitShare > 80 {
		a.Pattern = "lone_wolf"
		a.PatternDesc = "Dominated by a single contributor (>80% of commits)"
		return
	}

	if a.GiniCoefficient < 0.3 {
		a.Pattern = "balanced"
		a.PatternDesc = "Well-balanced contribution distribution"
		return
	}

	if a.GiniCoefficient < 0.5 {
		a.Pattern = "moderate_imbalance"
		a.PatternDesc = "Moderately imbalanced distribution"
		return
	}

	a.Pattern = "imbalanced"
	a.PatternDesc = "Highly imbalanced contribution distribution"
}

// calculateGini calculates the Gini coefficient for a distribution
// 0 = perfect equality, 1 = perfect inequality
func calculateGini(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}

	// Sort values
	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	// Calculate Gini
	var sum, cumSum float64
	for i, v := range sorted {
		cumSum += v
		sum += float64(i+1) * v
	}

	if cumSum == 0 {
		return 0
	}

	return (2*sum)/(float64(n)*cumSum) - (float64(n)+1)/float64(n)
}

// CalculateStandardDeviation calculates the standard deviation of commit counts
func CalculateStandardDeviation(commits []int) float64 {
	if len(commits) == 0 {
		return 0
	}

	// Calculate mean
	var sum float64
	for _, c := range commits {
		sum += float64(c)
	}
	mean := sum / float64(len(commits))

	// Calculate variance
	var variance float64
	for _, c := range commits {
		diff := float64(c) - mean
		variance += diff * diff
	}
	variance /= float64(len(commits))

	return math.Sqrt(variance)
}
