package analysis

import (
	"regexp"
	"strings"
)

// ConventionalCommit represents a parsed conventional commit
type ConventionalCommit struct {
	Type        string
	Scope       string
	Description string
	IsBreaking  bool
	IsValid     bool
}

var conventionalCommitRegex = regexp.MustCompile(`^(\w+)(?:\(([^)]+)\))?(!)?:\s*(.+)`)

var validTypes = map[string]bool{
	"feat":     true,
	"fix":      true,
	"docs":     true,
	"style":    true,
	"refactor": true,
	"perf":     true,
	"test":     true,
	"build":    true,
	"ci":       true,
	"chore":    true,
	"revert":   true,
}

// ParseConventionalCommit parses a commit message according to the Conventional Commits spec
func ParseConventionalCommit(message string) *ConventionalCommit {
	if message == "" {
		return &ConventionalCommit{IsValid: false}
	}

	// Get first line
	firstLine := strings.Split(message, "\n")[0]

	matches := conventionalCommitRegex.FindStringSubmatch(firstLine)
	if matches == nil {
		return &ConventionalCommit{IsValid: false}
	}

	commitType := strings.ToLower(matches[1])
	if !validTypes[commitType] {
		return &ConventionalCommit{IsValid: false}
	}

	return &ConventionalCommit{
		Type:        commitType,
		Scope:       matches[2],
		IsBreaking:  matches[3] == "!",
		Description: matches[4],
		IsValid:     true,
	}
}

// CommitQualityAnalysis contains the analysis of commit quality for a repository
type CommitQualityAnalysis struct {
	TotalCommits      int            `json:"total_commits"`
	ConventionalCount int            `json:"conventional_count"`
	ConventionalPct   float64        `json:"conventional_pct"`
	TypeDistribution  map[string]int `json:"type_distribution"`
	BreakingChanges   int            `json:"breaking_changes"`
	AverageMessageLen float64        `json:"average_message_length"`
	CommitsWithScope  int            `json:"commits_with_scope"`
}

// AnalyzeCommitQuality analyzes the quality of commits
func AnalyzeCommitQuality(messages []string) *CommitQualityAnalysis {
	analysis := &CommitQualityAnalysis{
		TotalCommits:     len(messages),
		TypeDistribution: make(map[string]int),
	}

	if len(messages) == 0 {
		return analysis
	}

	totalLen := 0
	for _, msg := range messages {
		totalLen += len(msg)

		cc := ParseConventionalCommit(msg)
		if cc.IsValid {
			analysis.ConventionalCount++
			analysis.TypeDistribution[cc.Type]++
			if cc.IsBreaking {
				analysis.BreakingChanges++
			}
			if cc.Scope != "" {
				analysis.CommitsWithScope++
			}
		}
	}

	analysis.ConventionalPct = float64(analysis.ConventionalCount) / float64(analysis.TotalCommits) * 100
	analysis.AverageMessageLen = float64(totalLen) / float64(analysis.TotalCommits)

	return analysis
}
