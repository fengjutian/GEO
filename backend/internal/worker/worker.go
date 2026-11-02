// Package worker runs durable queued scans. It is deliberately small and
// process-local for the MVP; the store's conditional claim makes concurrent
// loops safe within one server process.
package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/fengjutian/GEO/backend/internal/service"
	"github.com/fengjutian/GEO/backend/internal/store"
)

type Worker struct {
	service     *service.Service
	store       *store.Store
	concurrency int
	poll        time.Duration
}

func New(svc *service.Service, st *store.Store, concurrency int, poll time.Duration) *Worker {
	if concurrency < 1 {
		concurrency = 1
	}
	if poll <= 0 {
		poll = 750 * time.Millisecond
	}
	return &Worker{service: svc, store: st, concurrency: concurrency, poll: poll}
}

// Run blocks until ctx is cancelled. Each loop claims at most one queued scan
// at a time, so concurrency is bounded by the number of loops.
func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < w.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.loop(ctx)
		}()
	}
	wg.Wait()
}

func (w *Worker) loop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		scan, err := w.store.ClaimNextScan(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("scan worker: claim failed: %v", err)
			w.wait(ctx)
			continue
		}
		if scan == nil {
			w.wait(ctx)
			continue
		}
		log.Printf("scan worker: scan #%d started (site #%d)", scan.ID, scan.SiteID)
		w.process(ctx, scan.ID)
	}
}

func (w *Worker) process(ctx context.Context, scanID uint) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("scan worker: scan #%d panicked: %v", scanID, recovered)
			_ = w.store.MarkScanFailed(context.Background(), scanID, "WORKER_PANIC", "扫描执行异常", time.Now())
		}
	}()
	if err := w.service.ProcessScan(ctx, scanID); err != nil {
		log.Printf("scan worker: scan #%d failed: %v", scanID, err)
	}
}

func (w *Worker) wait(ctx context.Context) {
	timer := time.NewTimer(w.poll)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
