package domain

const (
	// RuleVersion is stored on every scan so old reports stay traceable when
	// rule behaviour changes.
	RuleVersion = "2026.11.1"
	// ScoreVersion identifies the scoring algorithm.
	ScoreVersion = "v1"
)

func PenaltyForSeverity(severity string) int {
	switch severity {
	case "critical":
		return 30
	case "high":
		return 18
	case "medium":
		return 10
	case "low":
		return 4
	default:
		return 0
	}
}
