package service

import (
	"context"
	"errors"

	corereport "github.com/fengjutian/geopilot-core/report"

	"github.com/fengjutian/GEO/backend/internal/store"
)

func (s *Service) HTMLReport(ctx context.Context, scanID uint) (string, error) {
	site, scan, findings, err := s.reportData(ctx, scanID)
	if err != nil {
		return "", err
	}
	return corereport.RenderHTML(site, scan, findings), nil
}

func (s *Service) JSONReport(ctx context.Context, scanID uint) ([]byte, error) {
	site, scan, findings, err := s.reportData(ctx, scanID)
	if err != nil {
		return nil, err
	}
	payload, err := corereport.MarshalJSON(site, scan, findings)
	if err != nil {
		return nil, internal("报告序列化失败", err)
	}
	return payload, nil
}

func (s *Service) reportData(ctx context.Context, scanID uint) (corereport.Site, corereport.Scan, []corereport.Finding, error) {
	scan, err := s.store.GetScan(ctx, scanID)
	if err != nil {
		return corereport.Site{}, corereport.Scan{}, nil, mapStoreError("扫描记录不存在", err)
	}
	site, err := s.store.GetSite(ctx, scan.SiteID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return corereport.Site{}, corereport.Scan{}, nil, notFound("扫描关联的网站不存在")
		}
		return corereport.Site{}, corereport.Scan{}, nil, internal("无法读取网站", err)
	}
	findings, err := s.store.FindingsByScan(ctx, scan.ID)
	if err != nil {
		return corereport.Site{}, corereport.Scan{}, nil, internal("无法读取扫描问题", err)
	}
	reportSite := corereport.Site{
		ID:          site.ID,
		Name:        site.Name,
		URL:         site.URL,
		Description: site.Description,
		CreatedAt:   site.CreatedAt,
		UpdatedAt:   site.UpdatedAt,
	}
	reportScan := corereport.Scan{
		ID:           scan.ID,
		SiteID:       scan.SiteID,
		Status:       scan.Status,
		TargetURL:    scan.TargetURL,
		FinalURL:     scan.FinalURL,
		HTTPStatus:   scan.HTTPStatus,
		Score:        scan.Score,
		Summary:      scan.Summary,
		ErrorCode:    scan.ErrorCode,
		ErrorMessage: scan.ErrorMessage,
		RuleVersion:  scan.RuleVersion,
		ScoreVersion: scan.ScoreVersion,
		PageCount:    scan.PageCount,
		StartedAt:    scan.StartedAt,
		FinishedAt:   scan.FinishedAt,
		CreatedAt:    scan.CreatedAt,
	}
	reportFindings := make([]corereport.Finding, 0, len(findings))
	for _, finding := range findings {
		reportFindings = append(reportFindings, corereport.Finding{
			ID:             finding.ID,
			SiteID:         finding.SiteID,
			ScanID:         finding.ScanID,
			Fingerprint:    finding.Fingerprint,
			Severity:       finding.Severity,
			RuleCode:       finding.RuleCode,
			Title:          finding.Title,
			URL:            finding.URL,
			Evidence:       finding.Evidence,
			Recommendation: finding.Recommendation,
			ScorePenalty:   finding.ScorePenalty,
			Status:         finding.Status,
			FirstSeenAt:    finding.FirstSeenAt,
			LastSeenAt:     finding.LastSeenAt,
			CreatedAt:      finding.CreatedAt,
			UpdatedAt:      finding.UpdatedAt,
		})
	}
	return reportSite, reportScan, reportFindings, nil
}
