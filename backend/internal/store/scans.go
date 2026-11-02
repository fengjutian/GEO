package store

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/fengjutian/GEO/backend/internal/domain"
)

func (s *Store) GetScan(ctx context.Context, id uint) (domain.Scan, error) {
	var scan domain.Scan
	err := s.db.WithContext(ctx).First(&scan, id).Error
	return scan, translateNotFound(err)
}

func (s *Store) LatestScan(ctx context.Context, siteID uint, status string) (*domain.Scan, error) {
	query := s.db.WithContext(ctx).Where("site_id = ?", siteID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var scan domain.Scan
	if err := query.Order("id desc").First(&scan).Error; err != nil {
		return nil, translateNotFound(err)
	}
	return &scan, nil
}

func (s *Store) CreateScan(ctx context.Context, scan *domain.Scan) error {
	return s.db.WithContext(ctx).Create(scan).Error
}

func (s *Store) CountRunningScans(ctx context.Context, siteID uint, since time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&domain.Scan{}).
		Where("site_id = ? AND status = ? AND started_at > ?", siteID, domain.ScanStatusRunning, since).
		Count(&count).Error
	return count, err
}

func (s *Store) SaveScanResult(ctx context.Context, input ScanSave) ([]domain.Finding, error) {
	findings := input.Findings
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(input.Scan).Error; err != nil {
			return err
		}
		for i := range findings {
			finding := &findings[i]
			if err := tx.Create(finding).Error; err != nil {
				return err
			}
			if ruleCode, reopened := input.Reopened[finding.Fingerprint]; reopened {
				if err := tx.Model(&domain.Finding{}).
					Where("site_id = ? AND fingerprint = ? AND id <> ?", finding.SiteID, finding.Fingerprint, finding.ID).
					Update("status", domain.IssueStatusOpen).Error; err != nil {
					return err
				}
				if err := tx.Create(&domain.IssueEvent{
					SiteID:      finding.SiteID,
					FindingID:   finding.ID,
					Fingerprint: finding.Fingerprint,
					RuleCode:    ruleCode,
					FromStatus:  domain.IssueStatusResolved,
					ToStatus:    domain.IssueStatusOpen,
					Note:        "重新扫描仍检测到该问题，已自动重新打开。",
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return findings, nil
}

func (s *Store) MarkScanFailed(ctx context.Context, scanID uint, code, message string, finished time.Time) error {
	return s.db.WithContext(ctx).Model(&domain.Scan{}).Where("id = ?", scanID).Updates(map[string]any{
		"status":        domain.ScanStatusFailed,
		"finished_at":   finished,
		"error_code":    code,
		"error_message": message,
	}).Error
}

func (s *Store) ListScans(ctx context.Context, siteID uint, limit int) ([]ScanWithCounts, error) {
	var scans []domain.Scan
	if err := s.db.WithContext(ctx).Where("site_id = ?", siteID).Order("id desc").Limit(limit).Find(&scans).Error; err != nil {
		return nil, err
	}
	views := make([]ScanWithCounts, 0, len(scans))
	for _, scan := range scans {
		total, high, err := s.CountScanFindings(ctx, scan.ID)
		if err != nil {
			return nil, err
		}
		views = append(views, ScanWithCounts{Scan: scan, IssueCount: total, HighIssueCount: high})
	}
	return views, nil
}

func (s *Store) PreviousCompletedScan(ctx context.Context, siteID, beforeID uint) (*domain.Scan, error) {
	var scan domain.Scan
	err := s.db.WithContext(ctx).
		Where("site_id = ? AND id < ? AND status = ?", siteID, beforeID, domain.ScanStatusCompleted).
		Order("id desc").First(&scan).Error
	if err != nil {
		return nil, translateNotFound(err)
	}
	return &scan, nil
}

func (s *Store) RecentScans(ctx context.Context, limit int) ([]domain.Scan, error) {
	var scans []domain.Scan
	err := s.db.WithContext(ctx).Order("id desc").Limit(limit).Find(&scans).Error
	return scans, err
}

func (s *Store) CountScans(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&domain.Scan{}).Count(&count).Error
	return count, err
}
