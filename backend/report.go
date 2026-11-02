package main

import (
	"fmt"
	"html"
	"strings"
)

func renderReport(site Site, scan Scan, findings []Finding) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>GeoPilot 诊断报告</title><style>body{font:16px/1.6 system-ui,sans-serif;max-width:1000px;margin:40px auto;padding:0 20px;color:#172033} .muted{color:#64748b}.score{font-size:48px;font-weight:700}.finding{border:1px solid #e2e8f0;border-radius:12px;padding:16px;margin:12px 0}.critical{color:#b91c1c}.high{color:#c2410c}.medium{color:#a16207}.low{color:#475569}code{overflow-wrap:anywhere}</style></head><body>`)
	b.WriteString("<h1>GeoPilot 网站诊断报告</h1>")
	b.WriteString("<p class='muted'>网站："+html.EscapeString(site.Name)+" · "+html.EscapeString(site.URL)+"</p>")
	b.WriteString(fmt.Sprintf("<div class='score'>%d / 100</div><p>%s</p>",scan.Score,html.EscapeString(scan.Summary)))
	b.WriteString(fmt.Sprintf("<h2>问题清单（%d）</h2>",len(findings)))
	for _, f := range findings {
		b.WriteString("<section class='finding'><strong class='"+html.EscapeString(f.Severity)+"'>"+html.EscapeString(strings.ToUpper(f.Severity))+"</strong> <strong>"+html.EscapeString(f.Title)+"</strong>")
		b.WriteString("<p class='muted'>规则："+html.EscapeString(f.RuleCode)+" · <code>"+html.EscapeString(f.URL)+"</code></p>")
		b.WriteString("<p><b>证据：</b>"+html.EscapeString(f.Evidence)+"</p><p><b>建议：</b>"+html.EscapeString(f.Recommendation)+"</p></section>")
	}
	b.WriteString("<p class='muted'>本报告为规则引擎诊断结果，不构成任何 AI 搜索平台收录或引用的保证。</p></body></html>")
	return b.String()
}
