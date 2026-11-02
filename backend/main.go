package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	loadDotEnv(".env")
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	db, err := openDatabase(getenv("DB_DSN", "geopilot:change_me_geopilot@tcp(127.0.0.1:3306)/geopilot?charset=utf8mb4&parseTime=True&loc=Local"))
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := db.AutoMigrate(&Site{}, &Scan{}, &Finding{}, &IssueEvent{}); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	backfillData(db)
	failInterruptedScans(db)

	if getenv("GIN_MODE", "") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := newRouter(db, allowedOrigins(getenv("CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173,http://localhost:4173,http://127.0.0.1:4173")))
	port := getenv("PORT", "8080")
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() {
		log.Printf("GeoPilot API listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func openDatabase(dsn string) (*gorm.DB, error) {
	var lastErr error
	for attempt := 1; attempt <= 20; attempt++ {
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
		if err == nil {
			sqlDB, err := db.DB()
			if err == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					return db, nil
				} else {
					sqlDB.Close()
					lastErr = pingErr
				}
			} else {
				lastErr = err
			}
		} else {
			lastErr = err
		}
		log.Printf("database not ready (attempt %d/20): %v", attempt, lastErr)
		time.Sleep(time.Second)
	}
	return nil, lastErr
}

func allowedOrigins(value string) map[string]bool {
	origins := make(map[string]bool)
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins[origin] = true
		}
	}
	return origins
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// loadDotEnv provides a dependency-free way to use the documented .env file.
// Real environment variables always win over values in the file.
func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}
