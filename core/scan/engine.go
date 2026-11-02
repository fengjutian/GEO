package scan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	defaultMaxResponseBytes = int64(2 << 20)
	defaultMaxRedirects     = 3
	defaultUserAgent        = "GeoPilotBot/0.2 (+https://github.com/fengjutian/GEO)"
	maxResourceBytes        = int64(64 * 1024)
)

// Engine executes a configurable list of rules against a fetched page. It is
// safe to reuse across requests.
type Engine struct {
	rules            []Rule
	userAgent        string
	timeout          time.Duration
	maxResponseBytes int64
	maxRedirects     int
}

type Option func(*Engine)

func WithRules(rules ...Rule) Option {
	return func(engine *Engine) {
		engine.rules = append([]Rule(nil), rules...)
	}
}

func WithUserAgent(userAgent string) Option {
	return func(engine *Engine) {
		if strings.TrimSpace(userAgent) != "" {
			engine.userAgent = strings.TrimSpace(userAgent)
		}
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(engine *Engine) {
		if timeout > 0 {
			engine.timeout = timeout
		}
	}
}

func WithMaxResponseBytes(limit int64) Option {
	return func(engine *Engine) {
		if limit > 0 {
			engine.maxResponseBytes = limit
		}
	}
}

// New builds an engine with the default rule set. Pass WithRules to replace it
// with a custom rule set, or append to DefaultRules for an extended set.
func New(options ...Option) *Engine {
	engine := &Engine{
		userAgent:        defaultUserAgent,
		timeout:          DefaultTimeout,
		maxResponseBytes: defaultMaxResponseBytes,
		maxRedirects:     defaultMaxRedirects,
	}
	for _, option := range options {
		option(engine)
	}
	if len(engine.rules) == 0 {
		engine.rules = DefaultRules()
	}
	return engine
}

// Rules returns a copy of the configured rule list.
func (e *Engine) Rules() []Rule {
	return append([]Rule(nil), e.rules...)
}

// Scan fetches a target and evaluates all configured rules.
func (e *Engine) Scan(ctx context.Context, target string) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	client := newSafeClient(e.maxRedirects, e.timeout)
	defer client.CloseIdleConnections()

	u, err := NormalizeTargetURL(target)
	if err != nil {
		return failedResult(targetErrorCode(err), "网站地址无法安全访问", err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return failedResult("URL_INVALID", "网站地址无效", "无法构造页面请求。")
	}
	req.Header.Set("User-Agent", e.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return failedResult(fetchErrorCode(err), "无法抓取网站首页", err.Error())
	}
	defer resp.Body.Close()
	finalURL := resp.Request.URL
	finalURLString := finalURL.String()
	httpStatus := resp.StatusCode

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, e.maxResponseBytes+1))
	if readErr != nil {
		result := failedResult("HTTP_FETCH_FAILED", "页面内容读取失败", readErr.Error())
		result.FinalURL = finalURLString
		result.HTTPStatus = httpStatus
		return result
	}
	truncated := int64(len(body)) > e.maxResponseBytes
	if truncated {
		body = body[:e.maxResponseBytes]
	}
	doc, parseErr := html.Parse(strings.NewReader(string(body)))
	page := &Page{
		RequestedURL:  u,
		FinalURL:      finalURL,
		StatusCode:    httpStatus,
		ContentType:   resp.Header.Get("Content-Type"),
		Body:          body,
		BodyTruncated: truncated,
		Document:      doc,
		Resources:     fetchResources(ctx, client, u, e.userAgent),
	}
	if parseErr != nil {
		page.ParseError = parseErr.Error()
	} else {
		page.Text = visibleText(doc)
	}
	return e.Analyze(ctx, page)
}

// Analyze evaluates the rule set against an already prepared page. Callers can
// use this to audit stored snapshots, run rules in tests, or inject metadata
// without performing another network request.
func (e *Engine) Analyze(ctx context.Context, page *Page) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	findings := make([]Finding, 0, 8)
	for _, rule := range e.rules {
		for _, finding := range rule.Evaluate(ctx, page) {
			if finding.ScorePenalty == 0 {
				finding.ScorePenalty = finding.Severity.Penalty()
			}
			findings = append(findings, finding)
		}
	}
	sortFindings(findings)
	finalURL := ""
	if page.FinalURL != nil {
		finalURL = page.FinalURL.String()
	}
	return Result{
		Status:     StatusCompleted,
		Score:      Score(findings),
		Summary:    summarizeFindings(findings),
		FinalURL:   finalURL,
		HTTPStatus: page.StatusCode,
		Findings:   findings,
	}
}

func fetchResources(ctx context.Context, client *http.Client, target *url.URL, userAgent string) map[string]Resource {
	origin := target.Scheme + "://" + target.Host
	resources := map[string]Resource{
		ResourceRobots:  fetchResource(ctx, client, origin+"/robots.txt", userAgent),
		ResourceSitemap: fetchResource(ctx, client, origin+"/sitemap.xml", userAgent),
	}
	return resources
}

func fetchResource(ctx context.Context, client *http.Client, checkURL, userAgent string) Resource {
	resource := Resource{URL: checkURL}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		resource.Error = err.Error()
		return resource
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/plain,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		resource.Error = err.Error()
		return resource
	}
	defer resp.Body.Close()
	resource.StatusCode = resp.StatusCode
	resource.ContentType = resp.Header.Get("Content-Type")
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResourceBytes))
	if err != nil {
		resource.Error = err.Error()
		return resource
	}
	resource.Body = body
	return resource
}

func failedResult(code, title, message string) Result {
	return Result{
		Status:       StatusFailed,
		Score:        0,
		Summary:      "扫描未能完成：" + message,
		ErrorCode:    code,
		ErrorMessage: message,
		Findings: []Finding{NewFinding(
			SeverityCritical,
			code,
			title,
			message,
			"确认网站可从公网访问、TLS 证书有效，且目标地址不是本机、内网或云元数据地址。",
		)},
	}
}

func targetErrorCode(err error) string {
	var targetErr *TargetError
	if errors.As(err, &targetErr) {
		return targetErr.Code
	}
	return "TARGET_BLOCKED"
}

func fetchErrorCode(err error) string {
	var targetErr *TargetError
	if errors.As(err, &targetErr) {
		return targetErr.Code
	}
	if errors.Is(err, errTargetBlocked) {
		return "TARGET_BLOCKED"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "FETCH_TIMEOUT"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "tls"), strings.Contains(message, "x509"), strings.Contains(message, "certificate"):
		return "TLS_ERROR"
	case strings.Contains(message, "no such host"), strings.Contains(message, "cannot be resolved"):
		return "DNS_RESOLUTION_FAILED"
	default:
		return "HTTP_FETCH_FAILED"
	}
}

func sortFindings(findings []Finding) {
	order := map[Severity]int{
		SeverityCritical: 0,
		SeverityHigh:     1,
		SeverityMedium:   2,
		SeverityLow:      3,
	}
	sort.SliceStable(findings, func(i, j int) bool {
		left, right := order[findings[i].Severity], order[findings[j].Severity]
		if left == right {
			return findings[i].RuleCode < findings[j].RuleCode
		}
		return left < right
	})
}

func summarizeFindings(findings []Finding) string {
	counts := map[Severity]int{}
	for _, finding := range findings {
		counts[finding.Severity]++
	}
	return fmt.Sprintf("完成首页诊断，共发现 %d 项问题（严重 %d / 高 %d / 中 %d / 低 %d）。评分仅代表 GeoPilot 内部规则结果。",
		len(findings), counts[SeverityCritical], counts[SeverityHigh], counts[SeverityMedium], counts[SeverityLow])
}
