package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// These tests exercise routing, persistence and report generation against a
// real MySQL instance. They are opt-in so the fast unit suite remains
// dependency-free:
//
//	$env:TEST_DB_DSN="user:pass@tcp(127.0.0.1:3306)/geopilot_test?charset=utf8mb4&parseTime=True&loc=Local"
//	go test ./...
func setupAPITest(t *testing.T) (*gorm.DB, *gin.Engine) {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN is not set; skipping MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	if err := db.AutoMigrate(&Site{}, &Scan{}, &Finding{}, &IssueEvent{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	if err := db.Where("1 = 1").Delete(&IssueEvent{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("1 = 1").Delete(&Finding{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("1 = 1").Delete(&Scan{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("1 = 1").Delete(&Site{}).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	return db, newRouter(db, map[string]bool{"http://localhost:5173": true})
}

func requestJSON(t *testing.T, router http.Handler, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
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
	var decoded map[string]any
	if recorder.Body.Len() > 0 && recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		_ = json.Unmarshal(recorder.Body.Bytes(), &decoded)
	}
	return recorder, decoded
}

func requestList(t *testing.T, router http.Handler, path string) []map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d body %s", path, recorder.Code, recorder.Body.String())
	}
	var decoded []map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	return decoded
}

func TestAPISiteCRUDAndValidation(t *testing.T) {
	_, router := setupAPITest(t)

	recorder, payload := requestJSON(t, router, http.MethodPost, "/api/sites", map[string]string{"name": "Bad", "url": "http://127.0.0.1"})
	if recorder.Code != http.StatusBadRequest || payload["code"] != "VALIDATION_ERROR" {
		t.Fatalf("expected validation error, got %d %v", recorder.Code, payload)
	}

	recorder, site := requestJSON(t, router, http.MethodPost, "/api/sites", map[string]string{"name": "Example", "url": "https://93.184.216.34", "description": "first"})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create site: %d %s", recorder.Code, recorder.Body.String())
	}
	id := uint(site["id"].(float64))

	recorder, updated := requestJSON(t, router, http.MethodPut, fmt.Sprintf("/api/sites/%d", id), map[string]string{"name": "Example 2", "url": "https://93.184.216.34/docs", "description": "second"})
	if recorder.Code != http.StatusOK || updated["name"] != "Example 2" {
		t.Fatalf("update site: %d %v", recorder.Code, updated)
	}

	recorder, detail := requestJSON(t, router, http.MethodGet, fmt.Sprintf("/api/sites/%d", id), nil)
	if recorder.Code != http.StatusOK || detail["site"] == nil {
		t.Fatalf("get site: %d %v", recorder.Code, detail)
	}

	recorder, _ = requestJSON(t, router, http.MethodDelete, fmt.Sprintf("/api/sites/%d", id), nil)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("delete site: %d", recorder.Code)
	}
	recorder, _ = requestJSON(t, router, http.MethodGet, fmt.Sprintf("/api/sites/%d", id), nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted site to be missing, got %d", recorder.Code)
	}
}

func seedTwoScans(t *testing.T, db *gorm.DB) (Site, Scan, Scan) {
	t.Helper()
	site := Site{Name: "Seeded", URL: "https://93.184.216.34", Description: "test"}
	if err := db.Create(&site).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-2 * time.Hour)
	first := now
	second := now.Add(time.Hour)
	scan1 := Scan{SiteID: site.ID, Status: ScanStatusCompleted, TargetURL: site.URL, StartedAt: first.Add(-time.Minute), FinishedAt: &first, PageCount: 1, Score: 60, Summary: "first", RuleVersion: RuleVersion, ScoreVersion: ScoreVersion}
	scan2 := Scan{SiteID: site.ID, Status: ScanStatusCompleted, TargetURL: site.URL, StartedAt: second.Add(-time.Minute), FinishedAt: &second, PageCount: 1, Score: 72, Summary: "second", RuleVersion: RuleVersion, ScoreVersion: ScoreVersion}
	if err := db.Create(&scan1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&scan2).Error; err != nil {
		t.Fatal(err)
	}
	findings := []Finding{
		{SiteID: site.ID, ScanID: scan1.ID, Fingerprint: "fp-a", Severity: "high", RuleCode: "RULE_A", Title: "A", URL: site.URL, Evidence: "old", Recommendation: "fix", ScorePenalty: 18, Status: IssueStatusOpen, FirstSeenAt: first, LastSeenAt: first},
		{SiteID: site.ID, ScanID: scan1.ID, Fingerprint: "fp-b", Severity: "medium", RuleCode: "RULE_B", Title: "B", URL: site.URL, Evidence: "old", Recommendation: "fix", ScorePenalty: 10, Status: IssueStatusOpen, FirstSeenAt: first, LastSeenAt: first},
		{SiteID: site.ID, ScanID: scan2.ID, Fingerprint: "fp-b", Severity: "medium", RuleCode: "RULE_B", Title: "B persists", URL: site.URL, Evidence: "new", Recommendation: "fix", ScorePenalty: 10, Status: IssueStatusOpen, FirstSeenAt: first, LastSeenAt: second},
		{SiteID: site.ID, ScanID: scan2.ID, Fingerprint: "fp-c", Severity: "high", RuleCode: "RULE_C", Title: `<script>alert(1)</script>`, URL: site.URL, Evidence: "new", Recommendation: "fix", ScorePenalty: 18, Status: IssueStatusOpen, FirstSeenAt: second, LastSeenAt: second},
	}
	if err := db.Create(&findings).Error; err != nil {
		t.Fatal(err)
	}
	return site, scan1, scan2
}

func TestAPIFindingsLifecycleAndReports(t *testing.T) {
	db, router := setupAPITest(t)
	site, scan1, scan2 := seedTwoScans(t, db)

	findings := requestList(t, router, fmt.Sprintf("/api/sites/%d/findings", site.ID))
	if len(findings) != 2 {
		t.Fatalf("expected latest scan's 2 findings, got %d", len(findings))
	}
	recorder, siteDetail := requestJSON(t, router, http.MethodGet, fmt.Sprintf("/api/sites/%d", site.ID), nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("site detail: %d", recorder.Code)
	}
	scans := siteDetail["scans"].([]any)
	firstScan := scans[0].(map[string]any)
	if firstScan["issueCount"].(float64) != 2 || firstScan["highIssueCount"].(float64) != 1 {
		t.Fatalf("scan counts should include resolved/ignored issues: %#v", firstScan)
	}
	var targetID uint
	for _, finding := range findings {
		if finding["ruleCode"] == "RULE_B" {
			targetID = uint(finding["id"].(float64))
		}
	}
	if targetID == 0 {
		t.Fatal("RULE_B finding not found")
	}
	recorder, updated := requestJSON(t, router, http.MethodPatch, fmt.Sprintf("/api/issues/%d", targetID), map[string]string{"status": IssueStatusResolved, "note": "fixed"})
	if recorder.Code != http.StatusOK || updated["status"] != IssueStatusResolved {
		t.Fatalf("update issue: %d %v", recorder.Code, updated)
	}
	var oldOccurrence Finding
	if err := db.Where("scan_id = ? AND fingerprint = ?", scan1.ID, "fp-b").First(&oldOccurrence).Error; err != nil {
		t.Fatal(err)
	}
	if oldOccurrence.Status != IssueStatusResolved {
		t.Fatalf("status should be propagated to earlier occurrences, got %s", oldOccurrence.Status)
	}
	var events int64
	db.Model(&IssueEvent{}).Where("finding_id = ?", targetID).Count(&events)
	if events != 1 {
		t.Fatalf("expected one issue event, got %d", events)
	}

	recorder, detail := requestJSON(t, router, http.MethodGet, fmt.Sprintf("/api/scans/%d", scan2.ID), nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("scan detail: %d", recorder.Code)
	}
	comparison := detail["comparison"].(map[string]any)
	if comparison["newCount"].(float64) != 1 || comparison["persistingCount"].(float64) != 1 || comparison["resolvedCount"].(float64) != 1 {
		t.Fatalf("unexpected comparison: %#v", comparison)
	}

	htmlReport := httptest.NewRecorder()
	router.ServeHTTP(htmlReport, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/scans/%d/report", scan2.ID), nil))
	if htmlReport.Code != http.StatusOK {
		t.Fatalf("html report: %d", htmlReport.Code)
	}
	if bytes.Contains(htmlReport.Body.Bytes(), []byte("<script>alert(1)</script>")) {
		t.Fatal("html report did not escape external content")
	}
	if !bytes.Contains(htmlReport.Body.Bytes(), []byte("&lt;script&gt;")) {
		t.Fatal("html report is missing escaped content")
	}

	recorder, document := requestJSON(t, router, http.MethodGet, fmt.Sprintf("/api/scans/%d/report.json", scan2.ID), nil)
	if recorder.Code != http.StatusOK || document["formatVersion"] != "1.0" {
		t.Fatalf("json report: %d %v", recorder.Code, document)
	}
	counts := document["severityCounts"].(map[string]any)
	if counts["high"].(float64) != 1 || counts["medium"].(float64) != 1 {
		t.Fatalf("unexpected severity counts: %#v", counts)
	}

	recorder, dashboard := requestJSON(t, router, http.MethodGet, "/api/dashboard", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("dashboard: %d", recorder.Code)
	}
	stats := dashboard["stats"].(map[string]any)
	if stats["siteCount"].(float64) != 1 || stats["openIssues"].(float64) != 1 || stats["highPriority"].(float64) != 1 {
		t.Fatalf("unexpected dashboard stats: %#v", stats)
	}

	recorder, _ = requestJSON(t, router, http.MethodDelete, fmt.Sprintf("/api/sites/%d", site.ID), nil)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("delete site: %d", recorder.Code)
	}
	var remaining int64
	db.Model(&Finding{}).Where("site_id = ?", site.ID).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("expected findings to be deleted, got %d", remaining)
	}
	db.Model(&IssueEvent{}).Where("site_id = ?", site.ID).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("expected issue events to be deleted, got %d", remaining)
	}
}
