package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/fengjutian/GEO/backend/internal/domain"
)

var (
	ErrNotFound   = errors.New("resource not found")
	ErrScanActive = errors.New("site already has an active scan")
)

type Store struct {
	db *gorm.DB
}

type ScanWithCounts struct {
	domain.Scan
	IssueCount     int64 `json:"issueCount"`
	HighIssueCount int64 `json:"highIssueCount"`
}

type FindingFilter struct {
	Severity string
	Status   string
	Query    string
}

type ScanSave struct {
	Scan     *domain.Scan
	Findings []domain.Finding
	// Reopened maps a fingerprint to its rule code. The store updates earlier
	// occurrences and records an IssueEvent in the same transaction.
	Reopened map[string]string
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	gormLogger := logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             300 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})
	var lastErr error
	for attempt := 1; attempt <= 20; attempt++ {
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: gormLogger})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					sqlDB.SetMaxOpenConns(20)
					sqlDB.SetMaxIdleConns(5)
					sqlDB.SetConnMaxLifetime(30 * time.Minute)
					return &Store{db: db}, nil
				} else {
					sqlDB.Close()
					lastErr = pingErr
				}
			} else {
				lastErr = dbErr
			}
		} else {
			lastErr = err
		}
		if attempt == 20 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("database did not become ready: %w", lastErr)
}

func (s *Store) Migrate() error {
	return s.db.AutoMigrate(&domain.Site{}, &domain.Scan{}, &domain.Finding{}, &domain.IssueEvent{})
}

func (s *Store) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func translateNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
