package scan

import (
	"strings"

	"golang.org/x/net/html"
)

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
