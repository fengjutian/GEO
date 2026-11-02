package scan

import (
	"context"
	"fmt"
	"strings"
)

// DefaultRules returns a fresh copy of the built-in GEO diagnostics. Consumers
// can append custom rules:
//
//	engine := scan.New(scan.WithRules(append(scan.DefaultRules(), myRule)...))
func DefaultRules() []Rule {
	return []Rule{
		RuleFunc{CodeName: "HTTP_STATUS", Fn: checkHTTPStatus},
		RuleFunc{CodeName: "BODY_LIMIT", Fn: checkBodyLimit},
		RuleFunc{CodeName: "CONTENT_TYPE", Fn: checkContentType},
		RuleFunc{CodeName: "HTML_PARSE", Fn: checkHTMLParse},
		RuleFunc{CodeName: "TITLE", Fn: checkTitle},
		RuleFunc{CodeName: "META_DESCRIPTION", Fn: checkMetaDescription},
		RuleFunc{CodeName: "CANONICAL", Fn: checkCanonical},
		RuleFunc{CodeName: "H1", Fn: checkH1},
		RuleFunc{CodeName: "JSON_LD", Fn: checkJSONLD},
		RuleFunc{CodeName: "CONTENT", Fn: checkContent},
		RuleFunc{CodeName: "RESOURCES", Fn: checkResources},
	}
}

func checkHTTPStatus(_ context.Context, page *Page) []Finding {
	if page.StatusCode >= 200 && page.StatusCode < 300 {
		return nil
	}
	finalURL := page.RequestedURL.String()
	if page.FinalURL != nil {
		finalURL = page.FinalURL.String()
	}
	return []Finding{NewFinding(
		SeverityHigh,
		"HTTP_STATUS",
		"首页 HTTP 状态异常",
		fmt.Sprintf("最终响应为 HTTP %d（%s）", page.StatusCode, finalURL),
		"确保首页返回 200；正确配置重定向与错误页面。",
	)}
}

func checkBodyLimit(_ context.Context, page *Page) []Finding {
	if !page.BodyTruncated {
		return nil
	}
	return []Finding{NewFinding(
		SeverityHigh,
		"BODY_LIMIT",
		"首页内容超过分析上限",
		fmt.Sprintf("响应体超过 %d KiB，仅分析前 %d KiB。", len(page.Body)/1024, len(page.Body)/1024),
		"精简首页 HTML，并把主要内容保留在首屏文档中。",
	)}
}

func checkContentType(_ context.Context, page *Page) []Finding {
	if page.IsHTML() {
		return nil
	}
	return []Finding{NewFinding(
		SeverityHigh,
		"CONTENT_TYPE_UNSUPPORTED",
		"首页不是 HTML 页面",
		"Content-Type: "+page.ContentType,
		"确保网站首页直接返回可解析的 HTML 文档，而不是文件下载或纯数据响应。",
	)}
}

func checkHTMLParse(_ context.Context, page *Page) []Finding {
	if page.ParseError == "" || !page.IsHTML() {
		return nil
	}
	return []Finding{NewFinding(
		SeverityCritical,
		"HTML_PARSE_FAILED",
		"页面 HTML 无法解析",
		page.ParseError,
		"检查页面是否为合法 HTML，并移除会破坏文档结构的脚本或模板片段。",
	)}
}

func checkTitle(_ context.Context, page *Page) []Finding {
	if !page.IsHTML() || page.Document == nil {
		return nil
	}
	title := textOfFirst(page.Document, "title")
	if title == "" {
		return []Finding{NewFinding(SeverityHigh, "TITLE_MISSING", "缺少页面标题", "未找到非空 title", "为首页设置清晰、独特且能概括页面主题的 title。")}
	}
	if len([]rune(title)) > 65 {
		return []Finding{NewFinding(SeverityLow, "TITLE_LONG", "页面标题可能过长", Truncate(title, 180), "把标题控制在约 60–65 个字符内，并优先表达页面核心主题。")}
	}
	return nil
}

func checkMetaDescription(_ context.Context, page *Page) []Finding {
	if !page.IsHTML() || page.Document == nil {
		return nil
	}
	if metaContent(page.Document, "description") != "" {
		return nil
	}
	return []Finding{NewFinding(SeverityMedium, "META_DESCRIPTION_MISSING", "缺少 Meta Description", "未找到非空 description", "添加准确描述页面价值的摘要，避免关键词堆砌。")}
}

func checkCanonical(_ context.Context, page *Page) []Finding {
	if !page.IsHTML() || page.Document == nil {
		return nil
	}
	if linkHref(page.Document, "canonical") != "" {
		return nil
	}
	return []Finding{NewFinding(SeverityMedium, "CANONICAL_MISSING", "缺少 Canonical 声明", "未找到 rel=canonical", "为可索引页面设置指向规范 URL 的 canonical。")}
}

func checkH1(_ context.Context, page *Page) []Finding {
	if !page.IsHTML() || page.Document == nil {
		return nil
	}
	count := countTag(page.Document, "h1")
	switch {
	case count == 0:
		return []Finding{NewFinding(SeverityMedium, "H1_MISSING", "缺少 H1 主标题", "未找到 h1", "使用一个清晰的主标题表达页面主题，并保持标题层级连续。")}
	case count > 1:
		return []Finding{NewFinding(SeverityLow, "H1_MULTIPLE", "存在多个 H1", fmt.Sprintf("发现 %d 个 h1", count), "检查标题层级，确保主主题突出且结构语义清晰。")}
	default:
		return nil
	}
}

func checkJSONLD(_ context.Context, page *Page) []Finding {
	if !page.IsHTML() || page.Document == nil {
		return nil
	}
	if countJSONLD(page.Document) > 0 {
		return nil
	}
	return []Finding{NewFinding(SeverityMedium, "SCHEMA_MISSING", "未发现 JSON-LD 结构化数据", "未找到 application/ld+json", "根据实际业务添加合适的 Organization、WebSite、Product 或 Article 等 Schema。")}
}

func checkContent(_ context.Context, page *Page) []Finding {
	if !page.IsHTML() || page.Document == nil {
		return nil
	}
	if len([]rune(page.Text)) >= 200 {
		return nil
	}
	return []Finding{NewFinding(SeverityMedium, "THIN_CONTENT", "首页可读正文较少", fmt.Sprintf("提取到约 %d 个字符", len([]rune(page.Text))), "补充可被抓取的文本内容：服务说明、产品信息、作者/组织信息与可验证的事实。")}
}

func checkResources(_ context.Context, page *Page) []Finding {
	resources := []struct {
		key  string
		code string
	}{
		{ResourceRobots, "ROBOTS_UNAVAILABLE"},
		{ResourceSitemap, "SITEMAP_UNAVAILABLE"},
	}
	findings := make([]Finding, 0, 2)
	for _, item := range resources {
		resource, exists := page.Resources[item.key]
		if !exists {
			continue
		}
		switch {
		case resource.Error != "":
			findings = append(findings, NewFinding(
				SeverityLow,
				item.code,
				"站点基础文件无法访问",
				resource.URL+"："+Truncate(resource.Error, 300),
				"检查该资源是否应该存在，并确保返回正确状态和内容类型。",
			))
		case resource.StatusCode < 200 || resource.StatusCode >= 300:
			findings = append(findings, NewFinding(
				SeverityLow,
				item.code,
				"站点基础文件无法访问",
				fmt.Sprintf("%s 返回 HTTP %d", resource.URL, resource.StatusCode),
				"若站点有可索引页面，建议正确配置 robots.txt 和 sitemap.xml。",
			))
		}
	}
	return findings
}

// RuleCodes returns the stable rule identifiers in the default rule set.
func RuleCodes() []string {
	codes := make([]string, 0, 11)
	for _, rule := range DefaultRules() {
		codes = append(codes, rule.Code())
	}
	return codes
}

// NormalizeRuleCode is a small helper for rule authors that want stable,
// upper-case identifiers independent of input formatting.
func NormalizeRuleCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
