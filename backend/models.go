package main

import "time"

const (
	// RuleVersion is stored on every scan so reports remain traceable when
	// the rule set changes. Bump it whenever rule behaviour changes.
	RuleVersion = "2026.11.1"
	// ScoreVersion identifies the scoring algorithm. It is intentionally
	// separate from RuleVersion: adding a rule and changing penalties are
	// different kinds of changes.
	ScoreVersion = "v1"

	ScanStatusRunning   = "running"
	ScanStatusCompleted = "completed"
	ScanStatusFailed    = "failed"

	IssueStatusOpen       = "open"
	IssueStatusInProgress = "in_progress"
	IssueStatusResolved   = "resolved"
	IssueStatusIgnored    = "ignored"
)

type Site struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:160;not null"`
	URL         string    `json:"url" gorm:"size:2048;not null"`
	Description string    `json:"description" gorm:"type:text"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Scan struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	SiteID       uint       `json:"siteId" gorm:"index;not null"`
	Status       string     `json:"status" gorm:"size:32;index"`
	TargetURL    string     `json:"targetUrl" gorm:"size:2048"`
	FinalURL     string     `json:"finalUrl" gorm:"size:2048"`
	HTTPStatus   int        `json:"httpStatus"`
	StartedAt    time.Time  `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	PageCount    int        `json:"pageCount"`
	Score        int        `json:"score"`
	Summary      string     `json:"summary" gorm:"type:text"`
	ErrorCode    string     `json:"errorCode,omitempty" gorm:"size:80"`
	ErrorMessage string     `json:"errorMessage,omitempty" gorm:"type:text"`
	RuleVersion  string     `json:"ruleVersion" gorm:"size:32"`
	ScoreVersion string     `json:"scoreVersion" gorm:"size:32"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type Finding struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	SiteID         uint      `json:"siteId" gorm:"index;not null"`
	ScanID         uint      `json:"scanId" gorm:"index;not null"`
	Fingerprint    string    `json:"fingerprint" gorm:"size:64;index"`
	Severity       string    `json:"severity" gorm:"size:16;index"`
	RuleCode       string    `json:"ruleCode" gorm:"size:80"`
	Title          string    `json:"title" gorm:"size:255"`
	URL            string    `json:"url" gorm:"size:2048"`
	Evidence       string    `json:"evidence" gorm:"type:text"`
	Recommendation string    `json:"recommendation" gorm:"type:text"`
	ScorePenalty   int       `json:"scorePenalty"`
	Status         string    `json:"status" gorm:"size:32;index;default:open"`
	FirstSeenAt    time.Time `json:"firstSeenAt"`
	LastSeenAt     time.Time `json:"lastSeenAt"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// IssueEvent records an explicit user-driven status change. Automatic
// re-opening after a re-scan is recorded too, so the history is auditable.
type IssueEvent struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	SiteID      uint      `json:"siteId" gorm:"index;not null"`
	FindingID   uint      `json:"findingId" gorm:"index;not null"`
	Fingerprint string    `json:"fingerprint" gorm:"size:64;index"`
	RuleCode    string    `json:"ruleCode" gorm:"size:80"`
	FromStatus  string    `json:"fromStatus" gorm:"size:32"`
	ToStatus    string    `json:"toStatus" gorm:"size:32"`
	Note        string    `json:"note" gorm:"type:text"`
	CreatedAt   time.Time `json:"createdAt"`
}
