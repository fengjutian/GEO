package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRenderHTMLEscapesExternalContent(t *testing.T) {
	now := time.Now()
	site := Site{ID: 1, Name: `<script>alert(1)</script>`, URL: `https://example.com/?x="><script>`}
	scan := Scan{ID: 2, Status: "completed", Score: 50, Summary: `<img src=x onerror=alert(1)>`, RuleVersion: "v1", FinishedAt: &now, HTTPStatus: 200, FinalURL: site.URL}
	findings := []Finding{{
		Severity:       "high",
		Title:          `<svg onload=alert(1)>`,
		RuleCode:       "X",
		URL:            site.URL,
		Evidence:       `</p><script>bad()</script>`,
		Recommendation: `<b>fix</b>`,
		Status:         "open",
		ScorePenalty:   18,
	}}
	output := RenderHTML(site, scan, findings)
	for _, raw := range []string{"<script>alert(1)</script>", "<img src=x", "<svg onload", "</p><script>bad()</script>"} {
		if strings.Contains(output, raw) {
			t.Errorf("report contains unescaped content %q", raw)
		}
	}
	for _, expected := range []string{"&lt;script&gt;", "严重", "高", "扫描编号", "规则版本", "HTTP 状态"} {
		if !strings.Contains(output, expected) {
			t.Errorf("report misses %q", expected)
		}
	}
}

func TestBuildDocument(t *testing.T) {
	findings := []Finding{{Severity: "critical"}, {Severity: "high"}, {Severity: "high"}, {Severity: "low"}}
	document := BuildDocument(Site{ID: 1}, Scan{ID: 2}, findings)
	if document.SeverityCounts.Critical != 1 || document.SeverityCounts.High != 2 || document.SeverityCounts.Medium != 0 || document.SeverityCounts.Low != 1 {
		t.Fatalf("unexpected counts: %#v", document.SeverityCounts)
	}
	if document.FormatVersion != "1.0" || document.Boundary == "" {
		t.Fatal("report metadata is incomplete")
	}
	payload, err := json.Marshal(document)
	if err != nil || !strings.Contains(string(payload), `"formatVersion":"1.0"`) {
		t.Fatalf("unexpected json: %s (%v)", payload, err)
	}
}

func TestMarshalJSON(t *testing.T) {
	payload, err := MarshalJSON(Site{ID: 1, Name: "Example"}, Scan{ID: 2, Status: "completed"}, []Finding{{Severity: "high"}})
	if err != nil {
		t.Fatal(err)
	}
	var decoded Document
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SeverityCounts.High != 1 {
		t.Fatalf("unexpected counts: %#v", decoded.SeverityCounts)
	}
}
