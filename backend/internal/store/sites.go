package store

import (
	"context"

	"gorm.io/gorm"

	"github.com/fengjutian/GEO/backend/internal/domain"
)

func (s *Store) ListSites(ctx context.Context) ([]domain.Site, error) {
	var sites []domain.Site
	err := s.db.WithContext(ctx).Order("created_at desc").Find(&sites).Error
	return sites, err
}

func (s *Store) GetSite(ctx context.Context, id uint) (domain.Site, error) {
	var site domain.Site
	err := s.db.WithContext(ctx).First(&site, id).Error
	return site, translateNotFound(err)
}

func (s *Store) CreateSite(ctx context.Context, site *domain.Site) error {
	return s.db.WithContext(ctx).Create(site).Error
}

func (s *Store) UpdateSite(ctx context.Context, site *domain.Site) error {
	return s.db.WithContext(ctx).Model(site).Updates(map[string]any{
		"name":        site.Name,
		"url":         site.URL,
		"description": site.Description,
	}).Error
}

func (s *Store) DeleteSite(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("site_id = ?", id).Delete(&domain.IssueEvent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("site_id = ?", id).Delete(&domain.Finding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("site_id = ?", id).Delete(&domain.Scan{}).Error; err != nil {
			return err
		}
		return tx.Delete(&domain.Site{}, id).Error
	})
}
