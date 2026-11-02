package store

import (
	"context"
	"log"
	"time"

	"github.com/fengjutian/GEO/backend/internal/domain"
)

// FailInterruptedScans marks scans that were running when the process stopped.
// With a single-process synchronous scanner this is safe during startup.
func (s *Store) FailInterruptedScans(ctx context.Context) {
	result := s.db.WithContext(ctx).Model(&domain.Scan{}).Where("status = ?", domain.ScanStatusRunning).Updates(map[string]any{
		"status":        domain.ScanStatusFailed,
		"finished_at":   time.Now(),
		"error_code":    "INTERRUPTED",
		"error_message": "服务重启前扫描仍在执行，已标记为失败，请重新运行诊断。",
	})
	if result.Error != nil {
		log.Printf("mark interrupted scans: %v", result.Error)
	}
}

// Backfill fills fields introduced after the initial schema so old rows stay
// usable. It is intentionally best-effort and idempotent.
func (s *Store) Backfill(ctx context.Context) {
	var scans []domain.Scan
	if err := s.db.WithContext(ctx).Where("target_url = '' OR rule_version = '' OR score_version = ''").Find(&scans).Error; err != nil {
		log.Printf("backfill scans: %v", err)
		return
	}
	siteURLs := map[uint]string{}
	for _, scan := range scans {
		updates := map[string]any{"rule_version": domain.RuleVersion, "score_version": domain.ScoreVersion}
		if scan.TargetURL == "" {
			if _, exists := siteURLs[scan.SiteID]; !exists {
				var site domain.Site
				if s.db.WithContext(ctx).First(&site, scan.SiteID).Error == nil {
					siteURLs[scan.SiteID] = site.URL
				}
			}
			if value := siteURLs[scan.SiteID]; value != "" {
				updates["target_url"] = value
			}
		}
		if err := s.db.WithContext(ctx).Model(&domain.Scan{}).Where("id = ?", scan.ID).Updates(updates).Error; err != nil {
			log.Printf("backfill scan %d: %v", scan.ID, err)
		}
	}

	var findings []domain.Finding
	if err := s.db.WithContext(ctx).
		Where("fingerprint = '' OR first_seen_at IS NULL OR last_seen_at IS NULL OR score_penalty = 0").
		Find(&findings).Error; err != nil {
		log.Printf("backfill findings: %v", err)
		return
	}
	for _, finding := range findings {
		first := finding.FirstSeenAt
		if first.IsZero() {
			first = finding.CreatedAt
		}
		last := finding.LastSeenAt
		if last.IsZero() {
			last = finding.CreatedAt
		}
		status := finding.Status
		if status == "" {
			status = domain.IssueStatusOpen
		}
		penalty := finding.ScorePenalty
		if penalty == 0 {
			penalty = domain.PenaltyForSeverity(finding.Severity)
		}
		updates := map[string]any{
			"first_seen_at": first,
			"last_seen_at":  last,
			"status":        status,
			"score_penalty": penalty,
		}
		if finding.Fingerprint == "" {
			updates["fingerprint"] = fingerprintFor(finding.SiteID, finding.URL, finding.RuleCode)
		}
		if err := s.db.WithContext(ctx).Model(&domain.Finding{}).Where("id = ?", finding.ID).Updates(updates).Error; err != nil {
			log.Printf("backfill finding %d: %v", finding.ID, err)
		}
	}
}

func fingerprintFor(siteID uint, pageURL, ruleCode string) string {
	return domain.Fingerprint(siteID, pageURL, ruleCode)
}
