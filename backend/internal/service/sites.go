package service

import (
	"context"
	"errors"
	"strings"

	corescan "github.com/fengjutian/geopilot-core/scan"

	"github.com/fengjutian/GEO/backend/internal/domain"
	"github.com/fengjutian/GEO/backend/internal/store"
)

func (s *Service) ListSites(ctx context.Context) ([]domain.Site, error) {
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		return nil, internal("无法读取网站列表", err)
	}
	return sites, nil
}

func (s *Service) CreateSite(ctx context.Context, input SiteInput) (domain.Site, error) {
	normalized, name, description, err := validateSiteInput(input)
	if err != nil {
		return domain.Site{}, err
	}
	site := domain.Site{Name: name, URL: normalized, Description: description}
	if err := s.store.CreateSite(ctx, &site); err != nil {
		return domain.Site{}, internal("创建网站失败", err)
	}
	return site, nil
}

func (s *Service) UpdateSite(ctx context.Context, id uint, input SiteInput) (domain.Site, error) {
	site, err := s.store.GetSite(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return domain.Site{}, notFound("网站不存在")
		}
		return domain.Site{}, internal("无法读取网站", err)
	}
	normalized, name, description, err := validateSiteInput(input)
	if err != nil {
		return domain.Site{}, err
	}
	site.Name = name
	site.URL = normalized
	site.Description = description
	if err := s.store.UpdateSite(ctx, &site); err != nil {
		return domain.Site{}, internal("更新网站失败", err)
	}
	updated, err := s.store.GetSite(ctx, id)
	if err != nil {
		return domain.Site{}, internal("更新后读取网站失败", err)
	}
	return updated, nil
}

func (s *Service) GetSiteDetail(ctx context.Context, id uint) (SiteDetail, error) {
	site, err := s.store.GetSite(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return SiteDetail{}, notFound("网站不存在")
		}
		return SiteDetail{}, internal("无法读取网站", err)
	}
	scans, err := s.store.ListScans(ctx, site.ID, 20)
	if err != nil {
		return SiteDetail{}, internal("无法读取扫描记录", err)
	}
	summaries := make([]ScanSummary, 0, len(scans))
	for _, scan := range scans {
		summaries = append(summaries, ScanSummary{
			Scan:           scan.Scan,
			IssueCount:     scan.IssueCount,
			HighIssueCount: scan.HighIssueCount,
		})
	}
	detail := SiteDetail{Site: site, Scans: summaries}
	if len(summaries) > 0 {
		detail.LatestScan = &summaries[0]
	}
	return detail, nil
}

func (s *Service) DeleteSite(ctx context.Context, id uint) error {
	if _, err := s.store.GetSite(ctx, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return notFound("网站不存在")
		}
		return internal("无法读取网站", err)
	}
	if err := s.store.DeleteSite(ctx, id); err != nil {
		return internal("删除网站失败，数据未被修改", err)
	}
	return nil
}

func validateSiteInput(input SiteInput) (normalized, name, description string, err error) {
	name = strings.TrimSpace(input.Name)
	description = strings.TrimSpace(input.Description)
	rawURL := strings.TrimSpace(input.URL)
	if name == "" || rawURL == "" {
		return "", "", "", validation("网站名称和地址不能为空")
	}
	if len([]rune(name)) > 160 {
		return "", "", "", validation("网站名称不能超过 160 个字符")
	}
	if len([]rune(description)) > 4000 {
		return "", "", "", validation("备注不能超过 4000 个字符")
	}
	normalized, targetErr := corescan.NormalizeTarget(rawURL)
	if targetErr != nil {
		return "", "", "", validation(targetErr.Error())
	}
	return normalized, name, description, nil
}

func (s *Service) Dashboard(ctx context.Context) (Dashboard, error) {
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		return Dashboard{}, internal("无法读取工作台数据", err)
	}
	scanCount, err := s.store.CountScans(ctx)
	if err != nil {
		return Dashboard{}, internal("无法读取扫描统计", err)
	}
	result := Dashboard{
		Stats: DashboardStats{SiteCount: len(sites), ScanCount: scanCount},
		Sites: make([]DashboardSite, 0, len(sites)),
	}
	sumScore, scoredSites := 0, 0
	for _, site := range sites {
		item := DashboardSite{Site: site}
		if scan, scanErr := s.store.LatestScan(ctx, site.ID, ""); scanErr == nil {
			item.LatestScan = scan
		} else if !errors.Is(scanErr, store.ErrNotFound) {
			return Dashboard{}, internal("无法读取最近扫描", scanErr)
		}
		if scan, scanErr := s.store.LatestScan(ctx, site.ID, domain.ScanStatusCompleted); scanErr == nil {
			open, high, countErr := s.store.CountActiveScanFindings(ctx, scan.ID)
			if countErr != nil {
				return Dashboard{}, internal("无法读取问题统计", countErr)
			}
			item.OpenIssues = open
			item.HighPriority = high
			result.Stats.OpenIssues += open
			result.Stats.HighPriority += high
			sumScore += scan.Score
			scoredSites++
		} else if !errors.Is(scanErr, store.ErrNotFound) {
			return Dashboard{}, internal("无法读取最近成功扫描", scanErr)
		}
		result.Sites = append(result.Sites, item)
	}
	if scoredSites > 0 {
		result.Stats.AverageScore = int(float64(sumScore)/float64(scoredSites) + 0.5)
	}
	recent, err := s.store.RecentScans(ctx, 8)
	if err != nil {
		return Dashboard{}, internal("无法读取最近扫描", err)
	}
	names := make(map[uint]string, len(sites))
	for _, site := range sites {
		names[site.ID] = site.Name
	}
	result.RecentScans = make([]DashboardRecentScan, 0, len(recent))
	for _, scan := range recent {
		result.RecentScans = append(result.RecentScans, DashboardRecentScan{Scan: scan, SiteName: names[scan.SiteID]})
	}
	return result, nil
}

func (s *Service) Health(ctx context.Context) error {
	return s.store.Ping(ctx)
}

func mapStoreError(message string, err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return notFound(message)
	}
	return internal(message, err)
}
