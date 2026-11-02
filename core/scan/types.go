// Package scan provides the reusable site-audit engine behind GeoPilot.
//
// The package intentionally has no dependency on the HTTP API, database or
// application configuration. It can be published as a private Go module and
// consumed by workers, CLIs and future services.
package scan

import (
	"context"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// Penalty returns the score deduction associated with a severity.
func (s Severity) Penalty() int {
	switch s {
	case SeverityCritical:
		return 30
	case SeverityHigh:
		return 18
	case SeverityMedium:
		return 10
	case SeverityLow:
		return 4
	default:
		return 0
	}
}

type Status string

const (
	// StatusCompleted means the page was fetched and analysed. Findings may
	// still be present; scan status and issue severity are different concepts.
	StatusCompleted Status = "completed"
	// StatusFailed means no usable page result was produced.
	StatusFailed Status = "failed"
)

type Finding struct {
	Severity       Severity `json:"severity"`
	RuleCode       string   `json:"ruleCode"`
	Title          string   `json:"title"`
	Evidence       string   `json:"evidence"`
	Recommendation string   `json:"recommendation"`
	ScorePenalty   int      `json:"scorePenalty"`
}

type Result struct {
	Status       Status    `json:"status"`
	Score        int       `json:"score"`
	Summary      string    `json:"summary"`
	ErrorCode    string    `json:"errorCode,omitempty"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	FinalURL     string    `json:"finalUrl,omitempty"`
	HTTPStatus   int       `json:"httpStatus"`
	Findings     []Finding `json:"findings"`
}

// Resource is a secondary file fetched for site-level rules, such as
// robots.txt or sitemap.xml.
type Resource struct {
	URL         string
	StatusCode  int
	ContentType string
	Body        []byte
	Error       string
}

const (
	ResourceRobots  = "robots"
	ResourceSitemap = "sitemap"
)

// Page is the complete, deterministic input handed to every rule. Keeping the
// fetch result and parsed document here makes rules easy to test and lets
// callers inject additional metadata later without changing rule signatures.
type Page struct {
	RequestedURL  *url.URL
	FinalURL      *url.URL
	StatusCode    int
	ContentType   string
	Body          []byte
	BodyTruncated bool
	Document      *html.Node
	Text          string
	ParseError    string
	Resources     map[string]Resource
}

// IsHTML reports whether the response is safe to evaluate with HTML rules.
func (p *Page) IsHTML() bool {
	contentType := strings.ToLower(p.ContentType)
	if contentType == "" {
		return true
	}
	return strings.Contains(contentType, "html") || strings.Contains(contentType, "xhtml")
}

// Rule is the extension point for new diagnostics. A rule should be pure:
// it receives a prepared Page and returns zero or more findings.
type Rule interface {
	Code() string
	Evaluate(ctx context.Context, page *Page) []Finding
}

// RuleFunc adapts a function into a Rule, which keeps small rules concise and
// allows downstream users to register custom checks without extra types.
type RuleFunc struct {
	CodeName string
	Fn       func(ctx context.Context, page *Page) []Finding
}

func (r RuleFunc) Code() string { return r.CodeName }

func (r RuleFunc) Evaluate(ctx context.Context, page *Page) []Finding {
	if r.Fn == nil {
		return nil
	}
	return r.Fn(ctx, page)
}

// NewFinding builds a finding and fills in the score penalty. It is exported so
// custom rule packages produce consistent records.
func NewFinding(severity Severity, code, title, evidence, recommendation string) Finding {
	return Finding{
		Severity:       severity,
		RuleCode:       code,
		Title:          title,
		Evidence:       Truncate(evidence, 1200),
		Recommendation: recommendation,
		ScorePenalty:   severity.Penalty(),
	}
}

// Score calculates the bounded 0–100 score used by reports.
func Score(findings []Finding) int {
	score := 100
	for _, finding := range findings {
		penalty := finding.ScorePenalty
		if penalty == 0 {
			penalty = finding.Severity.Penalty()
		}
		score -= penalty
	}
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

// Truncate limits evidence text without splitting a UTF-8 rune.
func Truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

// DefaultTimeout is used when no custom timeout is supplied.
const DefaultTimeout = 15 * time.Second
