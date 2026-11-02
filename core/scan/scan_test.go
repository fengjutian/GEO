package scan

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
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
			got, err := NormalizeTarget(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				var targetErr *TargetError
				if !errors.As(err, &targetErr) {
					t.Fatalf("expected TargetError, got %T", err)
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
		if !IsBlockedIP(net.ParseIP(value)) {
			t.Errorf("expected %s to be blocked", value)
		}
	}
	allowed := []string{"93.184.216.34", "8.8.8.8", "2606:4700:4700::1111"}
	for _, value := range allowed {
		if IsBlockedIP(net.ParseIP(value)) {
			t.Errorf("expected %s to be allowed", value)
		}
	}
}

func analyzeHTML(t *testing.T, body string) Result {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	target, _ := url.Parse("https://example.com")
	page := &Page{
		RequestedURL: target,
		FinalURL:     target,
		StatusCode:   200,
		ContentType:  "text/html; charset=utf-8",
		Document:     doc,
		Text:         visibleText(doc),
		Resources:    map[string]Resource{},
	}
	return New().Analyze(context.Background(), page)
}

func TestAnalyzeCompletePage(t *testing.T) {
	body := `<html><head>
	<title>Acme GEO Platform</title>
	<meta name="description" content="Acme helps teams understand how AI search systems read their websites.">
	<link rel="canonical" href="https://example.com/">
	<script type="application/ld+json">{"@context":"https://schema.org","@type":"Organization"}</script>
	</head><body><h1>Make your site understandable</h1><p>` + strings.Repeat("Useful public content for AI search systems. ", 8) + `</p></body></html>`
	result := analyzeHTML(t, body)
	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", result.Findings)
	}
	if result.Score != 100 || result.Status != StatusCompleted {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestAnalyzeEmptyPage(t *testing.T) {
	result := analyzeHTML(t, `<html><head></head><body></body></html>`)
	codes := map[string]bool{}
	for _, finding := range result.Findings {
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

func TestAnalyzeMultipleH1AndLongTitle(t *testing.T) {
	result := analyzeHTML(t, `<html><head><title>`+strings.Repeat("长", 66)+`</title></head><body><h1>one</h1><h1>two</h1></body></html>`)
	codes := map[string]bool{}
	for _, finding := range result.Findings {
		codes[finding.RuleCode] = true
	}
	if !codes["TITLE_LONG"] || !codes["H1_MULTIPLE"] {
		t.Fatalf("expected TITLE_LONG and H1_MULTIPLE, got %#v", codes)
	}
}

func TestAnalyzeSkipsHTMLRulesForNonHTML(t *testing.T) {
	target, _ := url.Parse("https://example.com/file.pdf")
	page := &Page{
		RequestedURL: target,
		FinalURL:     target,
		StatusCode:   200,
		ContentType:  "application/pdf",
		Resources:    map[string]Resource{},
	}
	result := New().Analyze(context.Background(), page)
	if len(result.Findings) != 1 || result.Findings[0].RuleCode != "CONTENT_TYPE_UNSUPPORTED" {
		t.Fatalf("expected only content type finding, got %#v", result.Findings)
	}
}

func TestCustomRuleIsExtensible(t *testing.T) {
	custom := RuleFunc{
		CodeName: "CUSTOM",
		Fn: func(context.Context, *Page) []Finding {
			return []Finding{{Severity: SeverityHigh, RuleCode: "CUSTOM_CHECK", Title: "custom"}}
		},
	}
	engine := New(WithRules(append(DefaultRules(), custom)...))
	target, _ := url.Parse("https://example.com")
	result := engine.Analyze(context.Background(), &Page{
		RequestedURL: target,
		FinalURL:     target,
		StatusCode:   204,
		ContentType:  "text/plain",
		Resources:    map[string]Resource{},
	})
	found := false
	for _, finding := range result.Findings {
		if finding.RuleCode == "CUSTOM_CHECK" {
			found = true
			if finding.ScorePenalty != 18 {
				t.Fatalf("custom finding penalty = %d, want 18", finding.ScorePenalty)
			}
		}
	}
	if !found {
		t.Fatal("custom rule output was not included")
	}
}

func TestScoreAndTruncate(t *testing.T) {
	findings := []Finding{
		NewFinding(SeverityCritical, "A", "a", "", ""),
		NewFinding(SeverityHigh, "B", "b", "", ""),
		NewFinding(SeverityMedium, "C", "c", "", ""),
		NewFinding(SeverityLow, "D", "d", "", ""),
	}
	if got := Score(findings); got != 38 {
		t.Fatalf("got score %d, want 38", got)
	}
	if got := Score([]Finding{{Severity: SeverityCritical}, {Severity: SeverityCritical}, {Severity: SeverityCritical}, {Severity: SeverityCritical}}); got != 0 {
		t.Fatalf("got score %d, want 0", got)
	}
	if got := Truncate("  中文内容  ", 20); got != "中文内容" {
		t.Fatalf("got %q", got)
	}
	if got := Truncate("123456", 3); got != "123…" {
		t.Fatalf("got %q", got)
	}
}

func TestErrorClassification(t *testing.T) {
	if got := targetErrorCode(&TargetError{Code: "DNS_RESOLUTION_FAILED"}); got != "DNS_RESOLUTION_FAILED" {
		t.Errorf("unexpected target code %s", got)
	}
	if got := fetchErrorCode(context.DeadlineExceeded); got != "FETCH_TIMEOUT" {
		t.Errorf("deadline should be FETCH_TIMEOUT, got %s", got)
	}
	if got := fetchErrorCode(errors.New("tls: failed to verify certificate: x509: certificate has expired")); got != "TLS_ERROR" {
		t.Errorf("certificate error should be TLS_ERROR, got %s", got)
	}
	if got := fetchErrorCode(errors.New("no such host")); got != "DNS_RESOLUTION_FAILED" {
		t.Errorf("dns error should be DNS_RESOLUTION_FAILED, got %s", got)
	}
	if got := fetchErrorCode(errTargetBlocked); got != "TARGET_BLOCKED" {
		t.Errorf("dial policy error should be TARGET_BLOCKED, got %s", got)
	}
}
