package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	corescan "github.com/fengjutian/geopilot-core/scan"

	"github.com/fengjutian/GEO/backend/internal/config"
	"github.com/fengjutian/GEO/backend/internal/httpapi"
	"github.com/fengjutian/GEO/backend/internal/service"
	"github.com/fengjutian/GEO/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DBDSN)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(); err != nil {
		return err
	}
	st.Backfill(ctx)
	st.FailInterruptedScans(ctx)

	if cfg.GINMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	scanner := corescan.New()
	svc := service.New(st, scanner)
	router := httpapi.NewRouter(svc, cfg.AllowedOrigin)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("GeoPilot API listening on :%s", cfg.Port)
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
