package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	maxResponseBytes = 2 << 20
	maxRedirects     = 3
	requestTimeout   = 15 * time.Second
	userAgent        = "GeoPilotBot/0.2 (+https://github.com/geopilot)"
)

var errTargetBlocked = errors.New("target address is outside the allowed public network range")

var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",
	"100.64.0.0/10",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"240.0.0.0/4",
	"255.255.255.255/32",
	"64:ff9b::/96",
	"64:ff9b:1::/48",
	"2001:db8::/32",
)

type RuleFinding struct {
	Severity       string
	RuleCode       string
	Title          string
	Evidence       string
	Recommendation string
	ScorePenalty   int
}

type ScanResult struct {
	Status       string
	Score        int
	Summary      string
	ErrorCode    string
	ErrorMessage string
	Findings     []RuleFinding
}

func mustPrefixes(values ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			panic(err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

func normalizeTarget(raw string) (string, error) {
	u, err := normalizeTargetURL(raw)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func normalizeTargetURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL")
	}
	if err := validateTargetURL(u); err != nil {
		return nil, err
	}
	u.Fragment = ""
	u.Host = canonicalHost(u)
	return u, nil
}

func canonicalHost(u *url.URL) string {
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	if port == "" {
		if strings.Contains(host, ":") {
			return "[" + host + "]"
		}
		return host
	}
	return net.JoinHostPort(host, port)
}

func validateTargetURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are allowed")
	}
	if u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("URL must contain a host and cannot include credentials")
	}
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		return fmt.Errorf("only ports 80 and 443 are allowed")
	}
	return validateHost(u.Hostname())
}

func validateHost(host string) error {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return fmt.Errorf("URL must contain a host")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "metadata.google.internal" || host == "instance-data" {
		return fmt.Errorf("local and cloud metadata targets are blocked")
	}
	var ips []net.IP
	if parsed := net.ParseIP(strings.Trim(host, "[]")); parsed != nil {
		ips = []net.IP{parsed}
	} else {
		resolved, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("host cannot be resolved")
		}
		ips = resolved
	}
	if len(ips) == 0 {
		return fmt.Errorf("host cannot be resolved")
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("private, local, and non-public IP targets are blocked")
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	if !addr.IsValid() || !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func safeClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 nil, // ignore HTTP_PROXY, which could bypass the target checks
		DialContext:           safeDialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects")
			}
			if err := validateTargetURL(req.URL); err != nil {
				return err
			}
			return nil
		},
	}
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid target address")
	}
	if port != "80" && port != "443" {
		return nil, errTargetBlocked
	}
	var ips []net.IP
	if parsed := net.ParseIP(host); parsed != nil {
		ips = []net.IP{parsed}
	} else {
		resolved, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("host cannot be resolved")
		}
		ips = resolved
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("host cannot be resolved")
	}
	// Reject the whole target if any answer is non-public. Otherwise a DNS
	// round-robin record could be used to reach an internal address.
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return nil, errTargetBlocked
		}
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("could not connect")
	}
	return nil, lastErr
}

func ScanWebsite(target string) ScanResult {
	client := safeClient()
	defer client.CloseIdleConnections()

	u, err := normalizeTargetURL(target)
	if err != nil {
		return failedResult("TARGET_BLOCKED", "网站地址无法安全访问", err.Error())
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return failedResult("URL_INVALID", "网站地址无效", "无法构造页面请求。")
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return failedResult("HTTP_FETCH_FAILED", "无法抓取网站首页", err.Error())
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if readErr != nil {
		return failedResult("HTTP_FETCH_FAILED", "页面内容读取失败", readErr.Error())
	}
	findings := []RuleFinding{}
	add := func(sev, code, title, evidence, rec string) {
		findings = append(findings, newFinding(sev, code, title, evidence, rec))
	}
	if len(body) > maxResponseBytes {
		body = body[:maxResponseBytes]
		add("high", "BODY_LIMIT", "首页内容超过分析上限", fmt.Sprintf("响应体超过 %d KiB，仅分析前 %d KiB。", maxResponseBytes/1024, maxResponseBytes/1024), "精简首页 HTML，并把主要内容保留在首屏文档中。")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		add("high", "HTTP_STATUS", "首页 HTTP 状态异常", fmt.Sprintf("最终响应为 HTTP %d（%s）", resp.StatusCode, u.String()), "确保首页返回 200；正确配置重定向与错误页面。")
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "html") && !strings.Contains(contentType, "xhtml") {
		add("high", "CONTENT_TYPE_UNSUPPORTED", "首页不是 HTML 页面", "Content-Type: "+resp.Header.Get("Content-Type"), "确保网站首页直接返回可解析的 HTML 文档，而不是文件下载或纯数据响应。")
	} else {
		findings = append(findings, analyzeHTML(body)...)
	}

	findings = append(findings, checkResources(client, u)...)
	sortFindings(findings)
	score := scoreFindings(findings)
	return ScanResult{
		Status:   ScanStatusCompleted,
		Score:    score,
		Summary:  summarizeFindings(findings),
		Findings: findings,
	}
}

func failedResult(code, title, message string) ScanResult {
	return ScanResult{
		Status:       ScanStatusFailed,
		Score:        0,
		Summary:      "扫描未能完成：" + message,
		ErrorCode:    code,
		ErrorMessage: message,
		Findings: []RuleFinding{newFinding("critical", code, title, message, "确认网站可从公网访问、TLS 证书有效，且目标地址不是本机、内网或云元数据地址。")},
	}
}

func analyzeHTML(body []byte) []RuleFinding {
	findings := []RuleFinding{}
	add := func(sev, code, title, evidence, rec string) {
		findings = append(findings, newFinding(sev, code, title, evidence, rec))
	}
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return []RuleFinding{newFinding("critical", "HTML_PARSE_FAILED", "页面 HTML 无法解析", err.Error(), "检查页面是否为合法 HTML，并移除会破坏文档结构的脚本或模板片段。")}
	}
	title := textOfFirst(doc, "title")
	desc := metaContent(doc, "description")
	canonical := linkHref(doc, "canonical")
	h1Count := countTag(doc, "h1")
	schemaCount := countJSONLD(doc)
	mainText := visibleText(doc)
	titleLength := len([]rune(title))
	if title == "" {
		add("high", "TITLE_MISSING", "缺少页面标题", "未找到非空 title", "为首页设置清晰、独特且能概括页面主题的 title。")
	} else if titleLength > 65 {
		add("low", "TITLE_LONG", "页面标题可能过长", truncateEvidence(title, 180), "把标题控制在约 60–65 个字符内，并优先表达页面核心主题。")
	}
	if desc == "" {
		add("medium", "META_DESCRIPTION_MISSING", "缺少 Meta Description", "未找到非空 description", "添加准确描述页面价值的摘要，避免关键词堆砌。")
	}
	if canonical == "" {
		add("medium", "CANONICAL_MISSING", "缺少 Canonical 声明", "未找到 rel=canonical", "为可索引页面设置指向规范 URL 的 canonical。")
	}
	if h1Count == 0 {
		add("medium", "H1_MISSING", "缺少 H1 主标题", "未找到 h1", "使用一个清晰的主标题表达页面主题，并保持标题层级连续。")
	}
	if h1Count > 1 {
		add("low", "H1_MULTIPLE", "存在多个 H1", fmt.Sprintf("发现 %d 个 h1", h1Count), "检查标题层级，确保主主题突出且结构语义清晰。")
	}
	if schemaCount == 0 {
		add("medium", "SCHEMA_MISSING", "未发现 JSON-LD 结构化数据", "未找到 application/ld+json", "根据实际业务添加合适的 Organization、WebSite、Product 或 Article 等 Schema。")
	}
	if len([]rune(mainText)) < 200 {
		add("medium", "THIN_CONTENT", "首页可读正文较少", fmt.Sprintf("提取到约 %d 个字符", len([]rune(mainText))), "补充可被抓取的文本内容：服务说明、产品信息、作者/组织信息与可验证的事实。")
	}
	return findings
}

func checkResources(client *http.Client, target *url.URL) []RuleFinding {
	findings := []RuleFinding{}
	origin := target.Scheme + "://" + target.Host
	for _, path := range []string{"/robots.txt", "/sitemap.xml"} {
		checkURL := origin + path
		req, err := http.NewRequest(http.MethodGet, checkURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "text/plain,*/*;q=0.8")
		resp, err := client.Do(req)
		if err != nil {
			findings = append(findings, newFinding("low", "RESOURCE_UNAVAILABLE", "站点基础文件无法访问", path+"："+truncateEvidence(err.Error(), 300), "检查该资源是否应该存在，并确保返回正确状态和内容类型。"))
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
		status := resp.StatusCode
		resp.Body.Close()
		if status < 200 || status >= 300 {
			findings = append(findings, newFinding("low", "RESOURCE_UNAVAILABLE", "站点基础文件无法访问", fmt.Sprintf("%s 返回 HTTP %d", path, status), "若站点有可索引页面，建议正确配置 robots.txt 和 sitemap.xml。"))
		}
	}
	return findings
}

func newFinding(severity, code, title, evidence, recommendation string) RuleFinding {
	return RuleFinding{
		Severity:       severity,
		RuleCode:       code,
		Title:          title,
		Evidence:       truncateEvidence(evidence, 1200),
		Recommendation: recommendation,
		ScorePenalty:   penaltyFor(severity),
	}
}

func penaltyFor(severity string) int {
	switch severity {
	case "critical":
		return 30
	case "high":
		return 18
	case "medium":
		return 10
	case "low":
		return 4
	default:
		return 0
	}
}

func scoreFindings(findings []RuleFinding) int {
	score := 100
	for _, finding := range findings {
		score -= finding.ScorePenalty
	}
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func summarizeFindings(findings []RuleFinding) string {
	counts := map[string]int{}
	for _, finding := range findings {
		counts[finding.Severity]++
	}
	return fmt.Sprintf("完成首页诊断，共发现 %d 项问题（严重 %d / 高 %d / 中 %d / 低 %d）。评分仅代表 GeoPilot 内部规则结果。",
		len(findings), counts["critical"], counts["high"], counts["medium"], counts["low"])
}

func sortFindings(findings []RuleFinding) {
	order := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
	sort.SliceStable(findings, func(i, j int) bool {
		left, right := order[findings[i].Severity], order[findings[j].Severity]
		if left == right {
			return findings[i].RuleCode < findings[j].RuleCode
		}
		return left < right
	})
}

func fingerprintFor(siteID uint, pageURL, ruleCode string) string {
	normalized := strings.ToLower(strings.TrimSpace(pageURL))
	normalized = strings.TrimSuffix(normalized, "/")
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%s", siteID, normalized, ruleCode)))
	return hex.EncodeToString(sum[:])
}

func truncateEvidence(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

func textOfFirst(n *html.Node, tag string) string {
	if n.Type == html.ElementNode && n.Data == tag {
		return strings.TrimSpace(nodeText(n))
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if v := textOfFirst(c, tag); v != "" {
			return v
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(" ")
		b.WriteString(nodeText(c))
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func metaContent(n *html.Node, name string) string {
	if n.Type == html.ElementNode && n.Data == "meta" {
		var metaName, content string
		for _, a := range n.Attr {
			if strings.EqualFold(a.Key, "name") {
				metaName = strings.ToLower(a.Val)
			}
			if strings.EqualFold(a.Key, "content") {
				content = a.Val
			}
		}
		if metaName == name {
			return strings.TrimSpace(content)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if v := metaContent(c, name); v != "" {
			return v
		}
	}
	return ""
}

func linkHref(n *html.Node, rel string) string {
	if n.Type == html.ElementNode && n.Data == "link" {
		var relValue, href string
		for _, a := range n.Attr {
			if strings.EqualFold(a.Key, "rel") {
				relValue = strings.ToLower(a.Val)
			}
			if strings.EqualFold(a.Key, "href") {
				href = a.Val
			}
		}
		if strings.Contains(" "+relValue+" ", " "+rel+" ") {
			return strings.TrimSpace(href)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if v := linkHref(c, rel); v != "" {
			return v
		}
	}
	return ""
}

func countTag(n *html.Node, tag string) int {
	count := 0
	if n.Type == html.ElementNode && n.Data == tag {
		count++
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		count += countTag(c, tag)
	}
	return count
}

func countJSONLD(n *html.Node) int {
	count := 0
	if n.Type == html.ElementNode && n.Data == "script" {
		for _, a := range n.Attr {
			if strings.EqualFold(a.Key, "type") && strings.EqualFold(a.Val, "application/ld+json") {
				count++
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		count += countJSONLD(c)
	}
	return count
}

func visibleText(n *html.Node) string {
	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "noscript" || n.Data == "svg" || n.Data == "template") {
		return ""
	}
	var b strings.Builder
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		b.WriteString(" ")
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(visibleText(c))
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
