package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	corescan "github.com/fengjutian/geopilot-core/scan"

	"github.com/fengjutian/GEO/backend/internal/domain"
	"github.com/fengjutian/GEO/backend/internal/httpapi"
	"github.com/fengjutian/GEO/backend/internal/service"
	"github.com/fengjutian/GEO/backend/internal/store"
	"github.com/fengjutian/GEO/backend/internal/worker"
)

type stubScanner struct {
	mu          sync.Mutex
	results     []corescan.Result
	calls       int
	block       chan struct{}
	started     chan struct{}
	startedOnce sync.Once
}

func (s *stubScanner) Scan(ctx context.Context, _ string) corescan.Result {
	if s.started != nil {
		s.startedOnce.Do(func() { close(s.started) })
	}
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return corescan.Result{Status: corescan.StatusFailed, ErrorCode: "CANCELLED"}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.results) == 0 {
		return corescan.Result{Status: corescan.StatusFailed, ErrorCode: "NO_STUB"}
	}
	index := s.calls
	if index >= len(s.results) {
		index = len(s.results) - 1
	}
	s.calls++
	return s.results[index]
}

func setupAPITest(t *testing.T) (*gorm.DB, *store.Store, *gin.Engine, *stubScanner) {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN is not set; skipping MySQL integration tests")
	}
	gdb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	st, err := store.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := st.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, model := range []any{&domain.IssueEvent{}, &domain.Finding{}, &domain.Scan{}, &domain.Site{}} {
		if err := gdb.Where("1 = 1").Delete(model).Error; err != nil {
			t.Fatal(err)
		}
	}
	gin.SetMode(gin.TestMode)
	scanner := &stubScanner{}
	svc := service.New(st, scanner)
	router := httpapi.NewRouter(svc, map[string]bool{"http://localhost:5173": true})
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		worker.New(svc, st, 2, 10*time.Millisecond).Run(workerCtx)
		close(workerDone)
	}()
	t.Cleanup(func() {
		cancelWorker()
		<-workerDone
		_ = st.Close()
	})
	return gdb, st, router, scanner
}

type response struct {
	status int
	body   []byte
}

func request(t *testing.T, router http.Handler, method, path string, body any) response {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return response{status: recorder.Code, body: recorder.Body.Bytes()}
}

func decode[T any](t *testing.T, value response) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(value.body, &result); err != nil {
		t.Fatalf("decode response (%d): %v\n%s", value.status, err, value.body)
	}
	return result
}

func newSite(t *testing.T, router http.Handler) domain.Site {
	t.Helper()
	created := request(t, router, http.MethodPost, "/api/sites", map[string]string{
		"name": "Example", "url": "https://93.184.216.34", "description": "test",
	})
	if created.status != http.StatusCreated {
		t.Fatalf("create site: %d %s", created.status, created.body)
	}
	return decode[domain.Site](t, created)
}

func startScan(t *testing.T, router http.Handler, siteID uint) domain.Scan {
	t.Helper()
	created := request(t, router, http.MethodPost, fmt.Sprintf("/api/sites/%d/scans", siteID), nil)
	if created.status != http.StatusAccepted {
		t.Fatalf("start scan: %d %s", created.status, created.body)
	}
	return decode[struct {
		Scan domain.Scan `json:"scan"`
	}](t, created).Scan
}

func waitForScan(t *testing.T, router http.Handler, scanID uint) service.ScanDetail {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result := request(t, router, http.MethodGet, fmt.Sprintf("/api/scans/%d", scanID), nil)
		if result.status != http.StatusOK {
			t.Fatalf("poll scan %d: %d %s", scanID, result.status, result.body)
		}
		detail := decode[service.ScanDetail](t, result)
		if domain.IsTerminalScanStatus(detail.Scan.Status) {
			return detail
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("scan %d did not finish in time", scanID)
	return service.ScanDetail{}
}

func TestAPISiteCRUDAndValidation(t *testing.T) {
	_, _, router, _ := setupAPITest(t)

	invalid := request(t, router, http.MethodPost, "/api/sites", map[string]string{"name": "Bad", "url": "http://127.0.0.1"})
	if invalid.status != http.StatusBadRequest {
		t.Fatalf("expected validation error, got %d %s", invalid.status, invalid.body)
	}
	var apiErr map[string]any
	if err := json.Unmarshal(invalid.body, &apiErr); err != nil || apiErr["code"] != "VALIDATION_ERROR" {
		t.Fatalf("unexpected error payload: %s", invalid.body)
	}

	site := newSite(t, router)
	updated := request(t, router, http.MethodPut, fmt.Sprintf("/api/sites/%d", site.ID), map[string]string{
		"name": "Example 2", "url": "https://93.184.216.34/docs", "description": "second",
	})
	if updated.status != http.StatusOK || decode[domain.Site](t, updated).Name != "Example 2" {
		t.Fatalf("update site: %d %s", updated.status, updated.body)
	}

	detail := request(t, router, http.MethodGet, fmt.Sprintf("/api/sites/%d", site.ID), nil)
	if detail.status != http.StatusOK {
		t.Fatalf("get site: %d", detail.status)
	}

	deleted := request(t, router, http.MethodDelete, fmt.Sprintf("/api/sites/%d", site.ID), nil)
	if deleted.status != http.StatusNoContent {
		t.Fatalf("delete site: %d", deleted.status)
	}
	missing := request(t, router, http.MethodGet, fmt.Sprintf("/api/sites/%d", site.ID), nil)
	if missing.status != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", missing.status)
	}
}

func TestAPIRejectsDuplicateActiveScan(t *testing.T) {
	_, _, router, scanner := setupAPITest(t)
	site := newSite(t, router)
	scanner.block = make(chan struct{})
	scanner.started = make(chan struct{})

	first := startScan(t, router, site.ID)
	select {
	case <-scanner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not claim the queued scan")
	}

	duplicate := request(t, router, http.MethodPost, fmt.Sprintf("/api/sites/%d/scans", site.ID), nil)
	if duplicate.status != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate active scan, got %d %s", duplicate.status, duplicate.body)
	}
	var apiErr map[string]any
	if err := json.Unmarshal(duplicate.body, &apiErr); err != nil || apiErr["code"] != "SCAN_IN_PROGRESS" {
		t.Fatalf("unexpected duplicate error: %s", duplicate.body)
	}

	close(scanner.block)
	detail := waitForScan(t, router, first.ID)
	if !domain.IsTerminalScanStatus(detail.Scan.Status) {
		t.Fatalf("scan did not reach a terminal state: %s", detail.Scan.Status)
	}
}

func TestAPIScanLifecycleComparisonAndReports(t *testing.T) {
	gdb, _, router, scanner := setupAPITest(t)
	scanner.results = []corescan.Result{
		{
			Status: corescan.StatusCompleted, Score: 72, Summary: "first",
			FinalURL: "https://93.184.216.34/", HTTPStatus: 200,
			Findings: []corescan.Finding{
				{Severity: corescan.SeverityHigh, RuleCode: "RULE_A", Title: "A", Evidence: "old", Recommendation: "fix", ScorePenalty: 18},
				{Severity: corescan.SeverityMedium, RuleCode: "RULE_B", Title: "B", Evidence: "old", Recommendation: "fix", ScorePenalty: 10},
			},
		},
		{
			Status: corescan.StatusCompleted, Score: 72, Summary: "second",
			FinalURL: "https://93.184.216.34/", HTTPStatus: 200,
			Findings: []corescan.Finding{
				{Severity: corescan.SeverityMedium, RuleCode: "RULE_B", Title: "B persists", Evidence: "new", Recommendation: "fix", ScorePenalty: 10},
				{Severity: corescan.SeverityHigh, RuleCode: "RULE_C", Title: `<script>alert(1)</script>`, Evidence: "new", Recommendation: "fix", ScorePenalty: 18},
			},
		},
		{
			Status: corescan.StatusFailed, ErrorCode: "TLS_ERROR", ErrorMessage: "certificate expired",
			Findings: []corescan.Finding{{Severity: corescan.SeverityCritical, RuleCode: "TLS_ERROR", Title: "TLS", Evidence: "certificate expired", Recommendation: "renew", ScorePenalty: 30}},
		},
	}
	site := newSite(t, router)

	firstScan := startScan(t, router, site.ID)
	if firstScan.Status != domain.ScanStatusQueued {
		t.Fatalf("new scan should be queued, got %s", firstScan.Status)
	}
	firstDetail := waitForScan(t, router, firstScan.ID)
	if firstDetail.Scan.Status != domain.ScanStatusCompleted || len(firstDetail.Findings) != 2 {
		t.Fatalf("unexpected first scan: %#v", firstDetail)
	}

	findings := decode[[]domain.Finding](t, request(t, router, http.MethodGet, fmt.Sprintf("/api/sites/%d/findings", site.ID), nil))
	if len(findings) != 2 {
		t.Fatalf("expected two findings, got %d", len(findings))
	}
	var ruleB domain.Finding
	for _, finding := range findings {
		if finding.RuleCode == "RULE_B" {
			ruleB = finding
		}
	}
	if ruleB.ID == 0 {
		t.Fatal("RULE_B not found")
	}
	resolved := request(t, router, http.MethodPatch, fmt.Sprintf("/api/issues/%d", ruleB.ID), map[string]string{"status": domain.IssueStatusResolved, "note": "fixed"})
	if resolved.status != http.StatusOK || decode[domain.Finding](t, resolved).Status != domain.IssueStatusResolved {
		t.Fatalf("resolve issue: %d %s", resolved.status, resolved.body)
	}
	var events int64
	gdb.Model(&domain.IssueEvent{}).Count(&events)
	if events != 1 {
		t.Fatalf("expected one manual status event, got %d", events)
	}

	secondScan := startScan(t, router, site.ID)
	secondDetail := waitForScan(t, router, secondScan.ID)
	var reopened domain.Finding
	for _, finding := range secondDetail.Findings {
		if finding.RuleCode == "RULE_B" {
			reopened = finding
		}
	}
	if reopened.Status != domain.IssueStatusOpen {
		t.Fatalf("resolved issue should reopen on recurrence, got %s", reopened.Status)
	}
	if reopened.FirstSeenAt.IsZero() {
		t.Fatal("first seen date was not preserved")
	}
	gdb.Model(&domain.IssueEvent{}).Count(&events)
	if events != 2 {
		t.Fatalf("expected automatic reopen event, got %d events", events)
	}

	detail := decode[service.ScanDetail](t, request(t, router, http.MethodGet, fmt.Sprintf("/api/scans/%d", secondScan.ID), nil))
	if detail.Comparison.NewCount != 1 || detail.Comparison.PersistingCount != 1 || detail.Comparison.ResolvedCount != 1 {
		t.Fatalf("unexpected comparison: %#v", detail.Comparison)
	}
	if detail.Scan.FinalURL == "" || detail.Scan.HTTPStatus != 200 {
		t.Fatalf("final URL / status missing: %#v", detail.Scan)
	}

	htmlReport := request(t, router, http.MethodGet, fmt.Sprintf("/api/scans/%d/report", secondScan.ID), nil)
	if htmlReport.status != http.StatusOK {
		t.Fatalf("html report: %d", htmlReport.status)
	}
	if bytes.Contains(htmlReport.body, []byte("<script>alert(1)</script>")) {
		t.Fatal("html report did not escape external content")
	}
	if !bytes.Contains(htmlReport.body, []byte("&lt;script&gt;")) {
		t.Fatal("html report is missing escaped content")
	}

	jsonReport := decode[map[string]any](t, request(t, router, http.MethodGet, fmt.Sprintf("/api/scans/%d/report.json", secondScan.ID), nil))
	if jsonReport["formatVersion"] != "1.0" {
		t.Fatalf("unexpected json report: %#v", jsonReport)
	}

	dashboard := decode[service.Dashboard](t, request(t, router, http.MethodGet, "/api/dashboard", nil))
	if dashboard.Stats.SiteCount != 1 || dashboard.Stats.OpenIssues != 2 || dashboard.Stats.HighPriority != 1 {
		t.Fatalf("unexpected dashboard stats: %#v", dashboard.Stats)
	}

	failedScan := startScan(t, router, site.ID)
	failedDetail := waitForScan(t, router, failedScan.ID)
	if failedDetail.Scan.Status != domain.ScanStatusFailed || failedDetail.Scan.ErrorCode != "TLS_ERROR" {
		t.Fatalf("failed scan should be recorded: %#v", failedDetail.Scan)
	}
	siteDetail := decode[service.SiteDetail](t, request(t, router, http.MethodGet, fmt.Sprintf("/api/sites/%d", site.ID), nil))
	if len(siteDetail.Scans) != 3 || siteDetail.Scans[0].Status != domain.ScanStatusFailed {
		t.Fatalf("unexpected scan history: %#v", siteDetail.Scans)
	}
	latestFindings := decode[[]domain.Finding](t, request(t, router, http.MethodGet, fmt.Sprintf("/api/sites/%d/findings", site.ID), nil))
	if len(latestFindings) != 2 {
		t.Fatalf("failed scans should not replace the latest successful findings, got %d", len(latestFindings))
	}

	deleted := request(t, router, http.MethodDelete, fmt.Sprintf("/api/sites/%d", site.ID), nil)
	if deleted.status != http.StatusNoContent {
		t.Fatalf("delete site: %d", deleted.status)
	}
	var remaining int64
	gdb.Model(&domain.Finding{}).Where("site_id = ?", site.ID).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("expected findings to be deleted, got %d", remaining)
	}
	gdb.Model(&domain.IssueEvent{}).Where("site_id = ?", site.ID).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("expected issue events to be deleted, got %d", remaining)
	}
}
