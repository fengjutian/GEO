package service

import (
	"context"
	"errors"
	"time"

	corescan "github.com/fengjutian/geopilot-core/scan"

	"github.com/fengjutian/GEO/backend/internal/domain"
	"github.com/fengjutian/GEO/backend/internal/store"
)

// EnqueueScan creates a durable queued job. The worker owns execution; the
// API returns as soon as the job is safely persisted.
func (s *Service) EnqueueScan(ctx context.Context, siteID uint) (domain.Scan, error) {
	site, err := s.store.GetSite(ctx, siteID)
	if err != nil {
		return domain.Scan{}, mapStoreError("网站不存在", err)
	}
	scan := domain.Scan{
		SiteID:       site.ID,
		Status:       domain.ScanStatusQueued,
		TargetURL:    site.URL,
		RuleVersion:  domain.RuleVersion,
		ScoreVersion: domain.ScoreVersion,
	}
	if err := s.store.CreateScanIfIdle(ctx, &scan); err != nil {
		switch {
		case errors.Is(err, store.ErrScanActive):
			return domain.Scan{}, conflict("SCAN_IN_PROGRESS", "该网站已有扫描正在排队或执行，请稍后重试")
		case errors.Is(err, store.ErrNotFound):
			return domain.Scan{}, notFound("网站不存在")
		default:
			return domain.Scan{}, internal("无法创建扫描任务", err)
		}
	}
	return scan, nil
}

// ProcessScan executes a scan already claimed by a worker (status=running).
// Persistence uses a short independent context so a shutting-down worker can
// still record the final state instead of leaving the job stuck.
func (s *Service) ProcessScan(ctx context.Context, scanID uint) error {
	scan, err := s.store.GetScan(ctx, scanID)
	if err != nil {
		return mapStoreError("扫描记录不存在", err)
	}
	if scan.Status != domain.ScanStatusRunning {
		if domain.IsTerminalScanStatus(scan.Status) {
			return nil
		}
		return internal("扫描任务状态不允许执行", nil)
	}
	site, err := s.store.GetSite(ctx, scan.SiteID)
	if err != nil {
		finished := time.Now()
		_ = s.store.MarkScanFailed(context.Background(), scan.ID, "SITE_NOT_FOUND", "扫描关联的网站不存在", finished)
		return mapStoreError("扫描关联的网站不存在", err)
	}

	result := s.scanner.Scan(ctx, site.URL)
	finished := time.Now()
	scan.FinishedAt = &finished
	scan.Status = string(result.Status)
	if scan.Status == "" {
		scan.Status = domain.ScanStatusFailed
	}
	scan.Score = result.Score
	scan.Summary = result.Summary
	scan.ErrorCode = result.ErrorCode
	scan.ErrorMessage = result.ErrorMessage
	scan.FinalURL = result.FinalURL
	scan.HTTPStatus = result.HTTPStatus
	if scan.Status == domain.ScanStatusCompleted {
		scan.PageCount = 1
	}

	persistCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	findings, reopened, err := s.buildFindings(persistCtx, site, scan.ID, result.Findings, finished)
	if err != nil {
		_ = s.store.MarkScanFailed(persistCtx, scan.ID, "PERSIST_FAILED", "扫描结果保存失败", finished)
		return internal("扫描已完成，但结果保存失败", err)
	}
	if _, err := s.store.SaveScanResult(persistCtx, store.ScanSave{Scan: &scan, Findings: findings, Reopened: reopened}); err != nil {
		_ = s.store.MarkScanFailed(persistCtx, scan.ID, "PERSIST_FAILED", "扫描结果保存失败", finished)
		return internal("扫描已完成，但结果保存失败", err)
	}
	return nil
}

func (s *Service) buildFindings(ctx context.Context, site domain.Site, scanID uint, items []corescan.Finding, now time.Time) ([]domain.Finding, map[string]string, error) {
	findings := make([]domain.Finding, 0, len(items))
	reopened := map[string]string{}
	for _, item := range items {
		fingerprint := domain.Fingerprint(site.ID, site.URL, item.RuleCode)
		status := domain.IssueStatusOpen
		firstSeen := now
		previous, err := s.store.FindLatestByFingerprint(ctx, site.ID, fingerprint)
		if err == nil {
			if previous.Status != "" {
				status = previous.Status
			}
			if !previous.FirstSeenAt.IsZero() {
				firstSeen = previous.FirstSeenAt
			} else if !previous.CreatedAt.IsZero() {
				firstSeen = previous.CreatedAt
			}
			if status == domain.IssueStatusResolved {
				status = domain.IssueStatusOpen
				reopened[fingerprint] = item.RuleCode
			}
		} else if !errors.Is(err, store.ErrNotFound) {
			return nil, nil, err
		}
		findings = append(findings, domain.Finding{
			SiteID:         site.ID,
			ScanID:         scanID,
			Fingerprint:    fingerprint,
			Severity:       string(item.Severity),
			RuleCode:       item.RuleCode,
			Title:          item.Title,
			URL:            site.URL,
			Evidence:       item.Evidence,
			Recommendation: item.Recommendation,
			ScorePenalty:   item.ScorePenalty,
			Status:         status,
			FirstSeenAt:    firstSeen,
			LastSeenAt:     now,
		})
	}
	return findings, reopened, nil
}

func (s *Service) SiteFindings(ctx context.Context, siteID, scanID uint, filter store.FindingFilter) ([]domain.Finding, error) {
	site, err := s.store.GetSite(ctx, siteID)
	if err != nil {
		return nil, mapStoreError("网站不存在", err)
	}
	if scanID == 0 {
		scan, err := s.store.LatestScan(ctx, site.ID, domain.ScanStatusCompleted)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return []domain.Finding{}, nil
			}
			return nil, internal("无法读取扫描记录", err)
		}
		scanID = scan.ID
	} else {
		scan, err := s.store.GetScan(ctx, scanID)
		if err != nil {
			return nil, mapStoreError("扫描记录不存在", err)
		}
		if scan.SiteID != site.ID {
			return nil, notFound("扫描记录不存在")
		}
	}
	findings, err := s.store.ListFindings(ctx, scanID, filter)
	if err != nil {
		return nil, internal("无法读取问题列表", err)
	}
	return findings, nil
}

func (s *Service) ScanDetail(ctx context.Context, scanID uint) (ScanDetail, error) {
	scan, err := s.store.GetScan(ctx, scanID)
	if err != nil {
		return ScanDetail{}, mapStoreError("扫描记录不存在", err)
	}
	site, err := s.store.GetSite(ctx, scan.SiteID)
	if err != nil {
		return ScanDetail{}, mapStoreError("扫描关联的网站不存在", err)
	}
	findings, err := s.store.FindingsByScan(ctx, scan.ID)
	if err != nil {
		return ScanDetail{}, internal("无法读取扫描问题", err)
	}
	comparison, err := s.compare(ctx, scan, findings)
	if err != nil {
		return ScanDetail{}, err
	}
	return ScanDetail{
		Scan:           scan,
		Site:           site,
		Findings:       findings,
		Comparison:     comparison,
		SeverityCounts: countBySeverity(findings),
		StatusCounts:   countByStatus(findings),
	}, nil
}

func (s *Service) compare(ctx context.Context, scan domain.Scan, current []domain.Finding) (Comparison, error) {
	comparison := Comparison{New: []domain.Finding{}, Resolved: []domain.Finding{}}
	if scan.Status != domain.ScanStatusCompleted {
		return comparison, nil
	}
	previous, err := s.store.PreviousCompletedScan(ctx, scan.SiteID, scan.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			comparison.New = append(comparison.New, current...)
			comparison.NewCount = len(current)
			return comparison, nil
		}
		return Comparison{}, internal("无法读取上一次扫描", err)
	}
	previousFindings, err := s.store.FindingsByScan(ctx, previous.ID)
	if err != nil {
		return Comparison{}, internal("无法读取上一次扫描问题", err)
	}
	comparison.BaseScanID = previous.ID
	currentByFingerprint := make(map[string]domain.Finding, len(current))
	for _, finding := range current {
		currentByFingerprint[finding.Fingerprint] = finding
	}
	previousByFingerprint := make(map[string]domain.Finding, len(previousFindings))
	for _, finding := range previousFindings {
		previousByFingerprint[finding.Fingerprint] = finding
		if _, exists := currentByFingerprint[finding.Fingerprint]; !exists {
			comparison.Resolved = append(comparison.Resolved, finding)
		}
	}
	for _, finding := range current {
		if _, exists := previousByFingerprint[finding.Fingerprint]; !exists {
			comparison.New = append(comparison.New, finding)
		}
	}
	comparison.NewCount = len(comparison.New)
	comparison.ResolvedCount = len(comparison.Resolved)
	comparison.PersistingCount = len(current) - comparison.NewCount
	return comparison, nil
}

func countBySeverity(findings []domain.Finding) map[string]int {
	counts := map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}
	for _, finding := range findings {
		counts[finding.Severity]++
	}
	return counts
}

func countByStatus(findings []domain.Finding) map[string]int {
	counts := map[string]int{
		domain.IssueStatusOpen:       0,
		domain.IssueStatusInProgress: 0,
		domain.IssueStatusResolved:   0,
		domain.IssueStatusIgnored:    0,
	}
	for _, finding := range findings {
		counts[finding.Status]++
	}
	return counts
}
