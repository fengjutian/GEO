// Package report renders GeoPilot audit results without depending on the
// application's database or HTTP layer. It can be reused by workers, CLIs and
// future report services.
package report

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"
)

type Site struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Scan struct {
	ID           uint       `json:"id"`
	SiteID       uint       `json:"siteId"`
	Status       string     `json:"status"`
	TargetURL    string     `json:"targetUrl"`
	FinalURL     string     `json:"finalUrl"`
	HTTPStatus   int        `json:"httpStatus"`
	Score        int        `json:"score"`
	Summary      string     `json:"summary"`
	ErrorCode    string     `json:"errorCode,omitempty"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	RuleVersion  string     `json:"ruleVersion"`
	ScoreVersion string     `json:"scoreVersion"`
	PageCount    int        `json:"pageCount"`
	StartedAt    time.Time  `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type Finding struct {
	ID             uint      `json:"id"`
	SiteID         uint      `json:"siteId"`
	ScanID         uint      `json:"scanId"`
	Fingerprint    string    `json:"fingerprint"`
	Severity       string    `json:"severity"`
	RuleCode       string    `json:"ruleCode"`
	Title          string    `json:"title"`
	URL            string    `json:"url"`
	Evidence       string    `json:"evidence"`
	Recommendation string    `json:"recommendation"`
	ScorePenalty   int       `json:"scorePenalty"`
	Status         string    `json:"status"`
	FirstSeenAt    time.Time `json:"firstSeenAt"`
	LastSeenAt     time.Time `json:"lastSeenAt"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type SeverityCounts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

type Document struct {
	FormatVersion  string         `json:"formatVersion"`
	GeneratedAt    time.Time      `json:"generatedAt"`
	Site           Site           `json:"site"`
	Scan           Scan           `json:"scan"`
	SeverityCounts SeverityCounts `json:"severityCounts"`
	Findings       []Finding      `json:"findings"`
	Boundary       string         `json:"boundary"`
}

const Boundary = "本报告由 GeoPilot 的确定性规则引擎生成，只反映扫描时获取到的公开页面信息。它不构成任何 AI 搜索平台收录、排名或引用的保证，也不能替代人工内容与业务审核。"

func CountBySeverity(findings []Finding) SeverityCounts {
	counts := SeverityCounts{}
	for _, finding := range findings {
		switch finding.Severity {
		case "critical":
			counts.Critical++
		case "high":
			counts.High++
		case "medium":
			counts.Medium++
		case "low":
			counts.Low++
		}
	}
	return counts
}

func BuildDocument(site Site, scan Scan, findings []Finding) Document {
	return Document{
		FormatVersion:  "1.0",
		GeneratedAt:    time.Now(),
		Site:           site,
		Scan:           scan,
		SeverityCounts: CountBySeverity(findings),
		Findings:       findings,
		Boundary:       Boundary,
	}
}

func MarshalJSON(site Site, scan Scan, findings []Finding) ([]byte, error) {
	return json.MarshalIndent(BuildDocument(site, scan, findings), "", "  ")
}

func RenderHTML(site Site, scan Scan, findings []Finding) string {
	var b strings.Builder
	counts := CountBySeverity(findings)
	finished := scan.FinishedAt
	if finished == nil || finished.IsZero() {
		finished = &scan.StartedAt
	}
	b.WriteString(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>GeoPilot 诊断报告</title><style>
:root{color-scheme:light}*{box-sizing:border-box}body{font:16px/1.65 "Segoe UI",system-ui,-apple-system,sans-serif;max-width:1040px;margin:40px auto;padding:0 24px;color:#172033;background:#fff}.muted{color:#64748b}.meta{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px;margin:24px 0}.meta div{border:1px solid #e2e8f0;border-radius:10px;padding:12px 14px}.meta b{display:block;font-size:12px;color:#64748b;font-weight:600}.score{font-size:56px;font-weight:750;line-height:1}.hero{display:flex;gap:32px;align-items:center;border:1px solid #e2e8f0;border-radius:16px;padding:24px;margin:24px 0;background:#f8fafc}.counts{display:flex;gap:8px;flex-wrap:wrap}.count{border-radius:999px;padding:5px 10px;font-size:13px;font-weight:650;background:#f1f5f9}.finding{border:1px solid #e2e8f0;border-radius:12px;padding:18px;margin:14px 0;break-inside:avoid}.finding h3{margin:6px 0 10px}.critical{color:#b91c1c}.high{color:#c2410c}.medium{color:#a16207}.low{color:#475569}.badge{display:inline-block;border-radius:6px;padding:2px 7px;font-size:12px;font-weight:700;background:#f1f5f9}.evidence{background:#f8fafc;border-left:3px solid #cbd5e1;padding:10px 12px;overflow-wrap:anywhere}code{overflow-wrap:anywhere;font-size:13px}h1{letter-spacing:-.02em}h2{margin-top:36px}.boundary{font-size:13px;color:#64748b;border-top:1px solid #e2e8f0;margin-top:32px;padding-top:16px}@media print{body{margin:0;max-width:none}.finding{break-inside:avoid}}
</style></head><body>`)
	b.WriteString("<h1>GeoPilot 网站诊断报告</h1>")
	b.WriteString("<p class='muted'>网站：" + html.EscapeString(site.Name) + " · <code>" + html.EscapeString(site.URL) + "</code></p>")
	b.WriteString("<div class='meta'>")
	b.WriteString(fmt.Sprintf("<div><b>扫描编号</b>#%d</div><div><b>扫描时间</b>%s</div><div><b>任务状态</b>%s</div><div><b>规则版本</b>%s</div>", scan.ID, html.EscapeString(finished.Local().Format("2006-01-02 15:04:05")), html.EscapeString(ScanStatusLabel(scan.Status)), html.EscapeString(scan.RuleVersion)))
	if scan.FinalURL != "" {
		b.WriteString("<div><b>最终 URL</b><code>" + html.EscapeString(scan.FinalURL) + "</code></div>")
	}
	if scan.HTTPStatus != 0 {
		b.WriteString(fmt.Sprintf("<div><b>HTTP 状态</b>%d</div>", scan.HTTPStatus))
	}
	b.WriteString("</div>")
	b.WriteString("<div class='hero'><div><div class='score'>" + fmt.Sprintf("%d", scan.Score) + "<span style='font-size:18px;font-weight:500;color:#64748b'> / 100</span></div><div class='muted' style='margin-top:6px'>GeoPilot 规则评分</div></div><div><p style='margin:0 0 12px'>" + html.EscapeString(scan.Summary) + "</p><div class='counts'>")
	b.WriteString(fmt.Sprintf("<span class='count critical'>严重 %d</span><span class='count high'>高 %d</span><span class='count medium'>中 %d</span><span class='count low'>低 %d</span>", counts.Critical, counts.High, counts.Medium, counts.Low))
	b.WriteString("</div></div></div>")
	if scan.ErrorMessage != "" {
		b.WriteString("<p class='evidence'><b>失败原因：</b>" + html.EscapeString(scan.ErrorMessage) + "</p>")
	}
	b.WriteString(fmt.Sprintf("<h2>问题清单（%d）</h2>", len(findings)))
	if len(findings) == 0 {
		b.WriteString("<p class='muted'>本次扫描未发现规则命中的问题。</p>")
	}
	for _, f := range findings {
		b.WriteString("<section class='finding'><span class='badge " + html.EscapeString(f.Severity) + "'>" + html.EscapeString(SeverityLabel(f.Severity)) + "</span> <span class='badge'>" + html.EscapeString(IssueStatusLabel(f.Status)) + "</span>")
		b.WriteString("<h3>" + html.EscapeString(f.Title) + "</h3>")
		b.WriteString("<p class='muted'>规则：<code>" + html.EscapeString(f.RuleCode) + "</code> · 扣分：" + fmt.Sprintf("%d", f.ScorePenalty) + " · <code>" + html.EscapeString(f.URL) + "</code></p>")
		b.WriteString("<p><b>证据</b></p><p class='evidence'>" + html.EscapeString(f.Evidence) + "</p>")
		b.WriteString("<p><b>整改建议</b></p><p>" + html.EscapeString(f.Recommendation) + "</p></section>")
	}
	b.WriteString("<p class='boundary'>" + html.EscapeString(Boundary) + "</p></body></html>")
	return b.String()
}

func SeverityLabel(severity string) string {
	switch severity {
	case "critical":
		return "严重"
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low":
		return "低"
	default:
		return severity
	}
}

func IssueStatusLabel(status string) string {
	switch status {
	case "open":
		return "待处理"
	case "in_progress":
		return "处理中"
	case "resolved":
		return "已修复"
	case "ignored":
		return "已忽略"
	default:
		return status
	}
}

func ScanStatusLabel(status string) string {
	switch status {
	case "running":
		return "扫描中"
	case "completed":
		return "已完成"
	case "failed":
		return "失败"
	default:
		return status
	}
}
