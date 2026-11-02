package service

import (
	"context"

	corescan "github.com/fengjutian/geopilot-core/scan"

	"github.com/fengjutian/GEO/backend/internal/domain"
	"github.com/fengjutian/GEO/backend/internal/store"
)

// Scanner is the core capability the application depends on. It is an
// interface so scans can be replaced by a queue-backed worker or a fake in
// tests without changing the service.
type Scanner interface {
	Scan(ctx context.Context, target string) corescan.Result
}

type Service struct {
	store   *store.Store
	scanner Scanner
}

func New(st *store.Store, scanner Scanner) *Service {
	if scanner == nil {
		scanner = corescan.New()
	}
	return &Service{store: st, scanner: scanner}
}

type SiteInput struct {
	Name        string
	URL         string
	Description string
}

// FindingFilter is the application-level filter for issue queries.
type FindingFilter = store.FindingFilter

type ScanSummary struct {
	domain.Scan
	IssueCount     int64 `json:"issueCount"`
	HighIssueCount int64 `json:"highIssueCount"`
}

type SiteDetail struct {
	Site       domain.Site   `json:"site"`
	Scans      []ScanSummary `json:"scans"`
	LatestScan *ScanSummary  `json:"latestScan,omitempty"`
}

type DashboardStats struct {
	SiteCount    int   `json:"siteCount"`
	ScanCount    int64 `json:"scanCount"`
	OpenIssues   int64 `json:"openIssues"`
	HighPriority int64 `json:"highPriority"`
	AverageScore int   `json:"averageScore"`
}

type DashboardSite struct {
	domain.Site
	LatestScan   *domain.Scan `json:"latestScan"`
	OpenIssues   int64        `json:"openIssues"`
	HighPriority int64        `json:"highPriority"`
}

type DashboardRecentScan struct {
	domain.Scan
	SiteName string `json:"siteName"`
}

type Dashboard struct {
	Stats       DashboardStats        `json:"stats"`
	Sites       []DashboardSite       `json:"sites"`
	RecentScans []DashboardRecentScan `json:"recentScans"`
}

type Comparison struct {
	BaseScanID      uint             `json:"baseScanId"`
	NewCount        int              `json:"newCount"`
	PersistingCount int              `json:"persistingCount"`
	ResolvedCount   int              `json:"resolvedCount"`
	New             []domain.Finding `json:"new"`
	Resolved        []domain.Finding `json:"resolved"`
}

type ScanDetail struct {
	Scan           domain.Scan      `json:"scan"`
	Site           domain.Site      `json:"site"`
	Findings       []domain.Finding `json:"findings"`
	Comparison     Comparison       `json:"comparison"`
	SeverityCounts map[string]int   `json:"severityCounts"`
	StatusCounts   map[string]int   `json:"statusCounts"`
}
