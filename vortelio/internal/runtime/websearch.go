package runtime

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// WebSearchResult is a single search hit.
type WebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// WebSearch queries DuckDuckGo's HTML endpoint (no API key required) and returns
// up to maxResults parsed results. It is intentionally dependency-light and tolerant
// of markup changes: it scans for the well-known result CSS classes.
func WebSearch(query string, maxResults int) ([]WebSearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	if maxResults <= 0 {
		maxResults = 5
	}

	form := url.Values{}
	form.Set("q", query)
	form.Set("kl", "")

	req, err := http.NewRequest("POST", "https://html.duckduckgo.com/html/", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	// DDG blocks requests without a browser-like UA.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("web search request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("web search returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}

	return parseDDG(string(body), maxResults), nil
}

// parseDDG walks the DuckDuckGo HTML result page.
func parseDDG(htmlStr string, maxResults int) []WebSearchResult {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil
	}

	var results []WebSearchResult
	var cur *WebSearchResult

	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if len(results) >= maxResults {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			class := attr(n, "class")
			switch {
			case strings.Contains(class, "result__a"):
				// New result: title + link.
				if cur != nil && cur.Title != "" {
					results = append(results, *cur)
				}
				href := attr(n, "href")
				if isAdLink(href) {
					cur = nil // skip sponsored results
					break
				}
				cur = &WebSearchResult{
					Title: cleanText(n),
					URL:   cleanDDGURL(href),
				}
			case strings.Contains(class, "result__snippet"):
				if cur != nil {
					cur.Snippet = cleanText(n)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if cur != nil && cur.Title != "" && len(results) < maxResults {
		results = append(results, *cur)
	}
	return results
}

// isAdLink reports whether a DuckDuckGo result link is a sponsored ad.
func isAdLink(href string) bool {
	return strings.Contains(href, "y.js") || strings.Contains(href, "ad_provider") || strings.Contains(href, "ad_domain")
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// cleanText extracts and normalizes all text under a node.
func cleanText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(sb.String()), " ")
}

// cleanDDGURL unwraps DuckDuckGo redirect links (//duckduckgo.com/l/?uddg=...).
func cleanDDGURL(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if uddg := u.Query().Get("uddg"); uddg != "" {
		return uddg
	}
	return raw
}
