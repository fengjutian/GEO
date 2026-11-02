package store

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/fengjutian/GEO/backend/internal/domain"
)

func (s *Store) FindingsByScan(ctx context.Context, scanID uint) ([]domain.Finding, error) {
	var findings []domain.Finding
	err := s.db.WithContext(ctx).Where("scan_id = ?", scanID).
		Order("FIELD(severity, 'critical', 'high', 'medium', 'low')").
		Order("id asc").Find(&findings).Error
	return findings, err
}

func (s *Store) ListFindings(ctx context.Context, scanID uint, filter FindingFilter) ([]domain.Finding, error) {
	query := s.db.WithContext(ctx).Where("scan_id = ?", scanID)
	if filter.Severity != "" {
		query = query.Where("severity = ?", filter.Severity)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(title LIKE ? OR rule_code LIKE ? OR evidence LIKE ?)", like, like, like)
	}
	var findings []domain.Finding
	err := query.Order("FIELD(severity, 'critical', 'high', 'medium', 'low')").Order("id asc").Find(&findings).Error
	return findings, err
}

func (s *Store) GetFinding(ctx context.Context, id uint) (domain.Finding, error) {
	var finding domain.Finding
	err := s.db.WithContext(ctx).First(&finding, id).Error
	return finding, translateNotFound(err)
}

func (s *Store) FindLatestByFingerprint(ctx context.Context, siteID uint, fingerprint string) (*domain.Finding, error) {
	var finding domain.Finding
	err := s.db.WithContext(ctx).
		Where("site_id = ? AND fingerprint = ?", siteID, fingerprint).
		Order("id desc").First(&finding).Error
	if err != nil {
		return nil, translateNotFound(err)
	}
	return &finding, nil
}

func (s *Store) CountScanFindings(ctx context.Context, scanID uint) (total, high int64, err error) {
	if err = s.db.WithContext(ctx).Model(&domain.Finding{}).Where("scan_id = ?", scanID).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	err = s.db.WithContext(ctx).Model(&domain.Finding{}).
		Where("scan_id = ? AND severity IN ?", scanID, []string{"critical", "high"}).
		Count(&high).Error
	return total, high, err
}

func (s *Store) CountActiveScanFindings(ctx context.Context, scanID uint) (open, high int64, err error) {
	active := []string{domain.IssueStatusOpen, domain.IssueStatusInProgress}
	if err = s.db.WithContext(ctx).Model(&domain.Finding{}).
		Where("scan_id = ? AND status IN ?", scanID, active).Count(&open).Error; err != nil {
		return 0, 0, err
	}
	err = s.db.WithContext(ctx).Model(&domain.Finding{}).
		Where("scan_id = ? AND status IN ? AND severity IN ?", scanID, active, []string{"critical", "high"}).
		Count(&high).Error
	return open, high, err
}

func (s *Store) UpdateIssueStatus(ctx context.Context, finding domain.Finding, status, note string) (domain.Finding, error) {
	from := finding.Status
	if from == "" {
		from = domain.IssueStatusOpen
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&domain.Finding{}).Where("site_id = ? AND fingerprint = ?", finding.SiteID, finding.Fingerprint)
		if finding.Fingerprint == "" {
			query = tx.Model(&domain.Finding{}).Where("id = ?", finding.ID)
		}
		if err := query.Updates(map[string]any{"status": status, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Create(&domain.IssueEvent{
			SiteID:      finding.SiteID,
			FindingID:   finding.ID,
			Fingerprint: finding.Fingerprint,
			RuleCode:    finding.RuleCode,
			FromStatus:  from,
			ToStatus:    status,
			Note:        note,
		}).Error
	})
	if err != nil {
		return domain.Finding{}, err
	}
	return s.GetFinding(ctx, finding.ID)
}
