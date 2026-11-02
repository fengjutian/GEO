package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type RuleFinding struct { Severity, RuleCode, Title, Evidence, Recommendation string }
type ScanResult struct { Score int; Summary string; Findings []RuleFinding }

func normalizeTarget(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil { return "", fmt.Errorf("invalid URL") }
	if u.Scheme == "" { raw = "https://" + raw; u, err = url.Parse(raw); if err != nil { return "", fmt.Errorf("invalid URL") } }
	if u.Scheme != "http" && u.Scheme != "https" { return "", fmt.Errorf("only http and https URLs are allowed") }
	if u.Hostname() == "" || u.User != nil { return "", fmt.Errorf("URL must contain a host and cannot include credentials") }
	if err := validateHost(u.Hostname()); err != nil { return "", err }
	u.Fragment = ""
	return u.String(), nil
}
func validateHost(host string) error {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") { return fmt.Errorf("localhost targets are blocked") }
	ips, err := net.LookupIP(host)
	if err != nil { return fmt.Errorf("host cannot be resolved") }
	for _, ip := range ips {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() { return fmt.Errorf("private, local, and non-public IP targets are blocked") }
	}
	return nil
}
func safeClient() *http.Client {
	return &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 { return fmt.Errorf("too many redirects") }
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" { return fmt.Errorf("unsupported redirect scheme") }
			if err := validateHost(req.URL.Hostname()); err != nil { return err }
			return nil
		},
	}
}
func ScanWebsite(target string) ScanResult {
	findings := []RuleFinding{}
	add := func(sev, code, title, evidence, rec string) { findings = append(findings, RuleFinding{sev,code,title,evidence,rec}) }
	u, err := normalizeTarget(target)
	if err != nil { add("critical","URL_INVALID","网站地址无法安全访问",err.Error(),"请使用可公开访问的 HTTP/HTTPS 网站地址。"); return ScanResult{0,"扫描未能完成",findings} }
	resp, err := safeClient().Get(u)
	if err != nil { add("critical","HTTP_FETCH_FAILED","无法抓取网站首页",err.Error(),"检查网站是否可从公网访问、TLS 证书是否有效，以及是否存在访问限制。"); return ScanResult{0,"首页抓取失败",findings} }
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if err != nil || len(body) > 2*1024*1024 { add("high","BODY_LIMIT","页面内容无法完整读取","响应体超过 2 MiB 或读取失败","压缩并精简 HTML，避免首页返回过大的文档。"); return ScanResult{40,"页面读取受限",findings} }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { add("high","HTTP_STATUS","首页 HTTP 状态异常",fmt.Sprintf("HTTP %d",resp.StatusCode),"确保重要页面返回 200；正确配置重定向与错误页面。") }
	doc, _ := html.Parse(strings.NewReader(string(body)))
	title := textOfFirst(doc, "title")
	desc := metaContent(doc, "description")
	canonical := linkHref(doc, "canonical")
	h1Count := countTag(doc, "h1")
	schemaCount := countJSONLD(doc)
	mainText := visibleText(doc)
	if title == "" { add("high","TITLE_MISSING","缺少页面标题","未找到非空 title","为首页设置清晰、独特且能概括页面主题的 title。") }
	if len([]rune(title)) > 65 { add("low","TITLE_LONG","页面标题可能过长",title,"压缩标题并将品牌名和页面核心主题放在优先位置。") }
	if desc == "" { add("medium","META_DESCRIPTION_MISSING","缺少 Meta Description","未找到非空 description","添加准确描述页面价值的摘要，避免关键词堆砌。") }
	if canonical == "" { add("medium","CANONICAL_MISSING","缺少 Canonical 声明","未找到 rel=canonical","为可索引页面设置指向规范 URL 的 canonical。") }
	if h1Count == 0 { add("medium","H1_MISSING","缺少 H1 主标题","未找到 h1","使用一个清晰的主标题表达页面主题。") }
	if h1Count > 1 { add("low","H1_MULTIPLE","存在多个 H1",fmt.Sprintf("发现 %d 个 h1",h1Count),"检查标题层级，确保主主题突出且结构语义清晰。") }
	if schemaCount == 0 { add("medium","SCHEMA_MISSING","未发现 JSON-LD 结构化数据","未找到 application/ld+json","根据实际业务添加合适的 Organization、WebSite、Product 或 Article 等 Schema。") }
	if len([]rune(mainText)) < 200 { add("medium","THIN_CONTENT","首页可读正文较少",fmt.Sprintf("提取到约 %d 个字符",len([]rune(mainText))),"补充可被抓取的文本内容：服务说明、产品信息、作者/组织信息与可验证的事实。") }
	base := strings.TrimRight(u, "/")
	for _, path := range []string{"/robots.txt","/sitemap.xml"} {
		checkURL := base + path
		req, _ := http.NewRequest("GET", checkURL, nil)
		r, e := safeClient().Do(req)
		if e != nil { add("low","RESOURCE_UNAVAILABLE","站点基础文件无法访问",path+"： "+e.Error(),"检查该资源是否应该存在，并确保返回正确状态和内容类型。"); continue }
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 64*1024))
		status := r.StatusCode; r.Body.Close()
		if status < 200 || status >= 300 { add("low","RESOURCE_UNAVAILABLE","站点基础文件无法访问",fmt.Sprintf("%s 返回 HTTP %d",path,status),"若站点有可索引页面，建议正确配置 robots.txt 和 sitemap.xml。") }
	}
	score := 100
	for _, f := range findings { switch f.Severity { case "critical": score -= 30; case "high": score -= 18; case "medium": score -= 10; case "low": score -= 4 } }
	if score < 0 { score = 0 }; if score > 100 { score = 100 }
	return ScanResult{score,fmt.Sprintf("完成首页诊断，共发现 %d 项待处理问题。当前评分仅为 GeoPilot 内部规则评分。",len(findings)),findings}
}
func textOfFirst(n *html.Node, tag string) string {
	if n.Type == html.ElementNode && n.Data == tag { return strings.TrimSpace(nodeText(n)) }
	for c:=n.FirstChild; c!=nil; c=c.NextSibling { if v:=textOfFirst(c,tag); v!="" { return v } }
	return ""
}
func nodeText(n *html.Node) string { var b strings.Builder; if n.Type==html.TextNode { b.WriteString(n.Data) }; for c:=n.FirstChild;c!=nil;c=c.NextSibling { b.WriteString(" "); b.WriteString(nodeText(c)) }; return strings.Join(strings.Fields(b.String())," ") }
func metaContent(n *html.Node, name string) string {
	if n.Type==html.ElementNode && n.Data=="meta" { var nm, content string; for _,a:=range n.Attr { if strings.EqualFold(a.Key,"name") { nm=strings.ToLower(a.Val) }; if strings.EqualFold(a.Key,"content") { content=a.Val } }; if nm=="description" { return strings.TrimSpace(content) } }
	for c:=n.FirstChild;c!=nil;c=c.NextSibling { if v:=metaContent(c,name); v!="" { return v } }; return ""
}
func linkHref(n *html.Node, rel string) string {
	if n.Type==html.ElementNode && n.Data=="link" { var r,h string; for _,a:=range n.Attr { if strings.EqualFold(a.Key,"rel") { r=strings.ToLower(a.Val) }; if a.Key=="href" { h=a.Val } }; if strings.Contains(" "+r+" "," "+rel+" ") { return strings.TrimSpace(h) } }
	for c:=n.FirstChild;c!=nil;c=c.NextSibling { if v:=linkHref(c,rel); v!="" { return v } }; return ""
}
func countTag(n *html.Node, tag string) int { x:=0; if n.Type==html.ElementNode && n.Data==tag { x++ }; for c:=n.FirstChild;c!=nil;c=c.NextSibling { x+=countTag(c,tag) }; return x }
func countJSONLD(n *html.Node) int {
	x:=0
	if n.Type==html.ElementNode && n.Data=="script" { for _,a:=range n.Attr { if strings.EqualFold(a.Key,"type") && strings.EqualFold(a.Val,"application/ld+json") { x++ } } }
	for c:=n.FirstChild;c!=nil;c=c.NextSibling { x+=countJSONLD(c) }; return x
}
func visibleText(n *html.Node) string {
	if n.Type==html.ElementNode && (n.Data=="script" || n.Data=="style" || n.Data=="noscript" || n.Data=="svg") { return "" }
	var b strings.Builder
	if n.Type==html.TextNode { b.WriteString(n.Data); b.WriteString(" ") }
	for c:=n.FirstChild;c!=nil;c=c.NextSibling { b.WriteString(visibleText(c)) }
	return strings.Join(strings.Fields(b.String())," ")
}
