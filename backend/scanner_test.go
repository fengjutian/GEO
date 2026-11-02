package main

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestNormalizeTarget(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "adds https", input: "93.184.216.34", want: "https://93.184.216.34"},
		{name: "keeps path", input: "https://93.184.216.34/docs", want: "https://93.184.216.34/docs"},
		{name: "removes default port", input: "https://93.184.216.34:443/docs", want: "https://93.184.216.34/docs"},
		{name: "removes fragment", input: "https://93.184.216.34/docs#top", want: "https://93.184.216.34/docs"},
		{name: "rejects ftp", input: "ftp://93.184.216.34", wantErr: true},
		{name: "rejects credentials", input: "https://user:pass@93.184.216.34", wantErr: true},
		{name: "rejects localhost", input: "http://localhost:8080", wantErr: true},
		{name: "rejects loopback", input: "http://127.0.0.1", wantErr: true},
		{name: "rejects private", input: "http://10.1.2.3", wantErr: true},
		{name: "rejects metadata", input: "http://169.254.169.254/latest/meta-data", wantErr: true},
		{name: "rejects mapped loopback", input: "http://[::ffff:127.0.0.1]", wantErr: true},
		{name: "rejects non standard port", input: "https://93.184.216.34:8443", wantErr: true},
		{name: "rejects empty", input: "   ", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTarget(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254",
		"100.64.0.1", "0.0.0.0", "192.0.2.10", "198.18.0.1", "224.0.0.1", "255.255.255.255",
		"::1", "fc00::1", "fe80::1", "::ffff:10.0.0.1", "2001:db8::1",
	}
	for _, value := range blocked {
		if !isBlockedIP(net.ParseIP(value)) {
			t.Errorf("expected %s to be blocked", value)
		}
	}
	allowed := []string{"93.184.216.34", "8.8.8.8", "2606:4700:4700::1111"}
	for _, value := range allowed {
		if isBlockedIP(net.ParseIP(value)) {
			t.Errorf("expected %s to be allowed", value)
		}
	}
}

func TestAnalyzeHTMLCompletePage(t *testing.T) {
	body := `<html><head>
	<title>Acme GEO Platform</title>
	<meta name="description" content="Acme helps teams understand how AI search systems read their websites.">
	<link rel="canonical" href="https://example.com/">
	<script type="application/ld+json">{"@context":"https://schema.org","@type":"Organization"}</script>
	</head><body><h1>Make your site understandable</h1><p>` + strings.Repeat("Useful public content for AI search systems. ", 8) + `</p></body></html>`
	findings := analyzeHTML([]byte(body))
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
	}
}

func TestAnalyzeHTMLEmptyPage(t *testing.T) {
	findings := analyzeHTML([]byte(`<html><head></head><body></body></html>`))
	codes := map[string]bool{}
	for _, finding := range findings {
		codes[finding.RuleCode] = true
		if finding.ScorePenalty <= 0 {
			t.Errorf("finding %s has no penalty", finding.RuleCode)
		}
	}
	for _, code := range []string{"TITLE_MISSING", "META_DESCRIPTION_MISSING", "CANONICAL_MISSING", "H1_MISSING", "SCHEMA_MISSING", "THIN_CONTENT"} {
		if !codes[code] {
			t.Errorf("expected finding %s, got %#v", code, codes)
		}
	}
}

func TestAnalyzeHTMLMultipleH1AndLongTitle(t *testing.T) {
	body := `<html><head><title>` + strings.Repeat("长", 66) + `</title></head><body><h1>one</h1><h1>two</h1></body></html>`
	findings := analyzeHTML([]byte(body))
	codes := map[string]bool{}
	for _, finding := range findings {
		codes[finding.RuleCode] = true
	}
	if !codes["TITLE_LONG"] || !codes["H1_MULTIPLE"] {
		t.Fatalf("expected TITLE_LONG and H1_MULTIPLE, got %#v", codes)
	}
}

func TestScoreFindings(t *testing.T) {
	findings := []RuleFinding{
		newFinding("critical", "A", "a", "", ""),
		newFinding("high", "B", "b", "", ""),
		newFinding("medium", "C", "c", "", ""),
		newFinding("low", "D", "d", "", ""),
	}
	if got := scoreFindings(findings); got != 38 {
		t.Fatalf("got score %d, want 38", got)
	}
	if got := scoreFindings([]RuleFinding{newFinding("critical", "A", "a", "", ""), newFinding("critical", "B", "b", "", "")}); got != 40 {
		t.Fatalf("got score %d, want 40", got)
	}
	if got := scoreFindings([]RuleFinding{newFinding("critical", "A", "a", "", ""), newFinding("critical", "B", "b", "", ""), newFinding("critical", "C", "c", "", ""), newFinding("critical", "D", "d", "", "")}); got != 0 {
		t.Fatalf("got score %d, want 0", got)
	}
}

func TestFingerprintFor(t *testing.T) {
	a := fingerprintFor(1, "HTTPS://Example.com/", "TITLE_MISSING")
	b := fingerprintFor(1, "https://example.com", "TITLE_MISSING")
	c := fingerprintFor(1, "https://example.com", "H1_MISSING")
	d := fingerprintFor(2, "https://example.com", "TITLE_MISSING")
	if a != b {
		t.Fatal("normalized URLs should share a fingerprint")
	}
	if a == c || a == d {
		t.Fatal("rule or site changes must produce a different fingerprint")
	}
	if len(a) != 64 {
		t.Fatalf("unexpected fingerprint length %d", len(a))
	}
}

func TestRenderReportEscapesExternalContent(t *testing.T) {
	site := Site{ID: 1, Name: `<script>alert(1)</script>`, URL: `https://example.com/?x="><script>`}
	scan := Scan{ID: 2, Status: ScanStatusCompleted, Score: 50, Summary: `<img src=x onerror=alert(1)>`, RuleVersion: RuleVersion, FinishedAt: func() *time.Time { now := time.Now(); return &now }()}
	findings := []Finding{{Severity: "high", Title: `<svg onload=alert(1)>`, RuleCode: "X", URL: site.URL, Evidence: `</p><script>bad()</script>`, Recommendation: `<b>fix</b>`, Status: IssueStatusOpen, ScorePenalty: 18}}
	output := renderReport(site, scan, findings)
	for _, raw := range []string{"<script>alert(1)</script>", "<img src=x", "<svg onload", "</p><script>bad()</script>"} {
		if strings.Contains(output, raw) {
			t.Errorf("report contains unescaped content %q", raw)
		}
	}
	for _, expected := range []string{"&lt;script&gt;", "严重", "高", "扫描编号", "规则版本"} {
		if !strings.Contains(output, expected) {
			t.Errorf("report misses %q", expected)
		}
	}
}

func TestBuildReportDocument(t *testing.T) {
	findings := []Finding{{Severity: "critical"}, {Severity: "high"}, {Severity: "high"}, {Severity: "low"}}
	document := buildReportDocument(Site{ID: 1}, Scan{ID: 2}, findings)
	if document.SeverityCounts.Critical != 1 || document.SeverityCounts.High != 2 || document.SeverityCounts.Medium != 0 || document.SeverityCounts.Low != 1 {
		t.Fatalf("unexpected counts: %#v", document.SeverityCounts)
	}
	if document.FormatVersion != "1.0" || document.Boundary == "" {
		t.Fatal("report metadata is incomplete")
	}
}

func TestTruncateEvidence(t *testing.T) {
	if got := truncateEvidence("  中文内容  ", 20); got != "中文内容" {
		t.Fatalf("got %q", got)
	}
	if got := truncateEvidence("123456", 3); got != "123…" {
		t.Fatalf("got %q", got)
	}
}
