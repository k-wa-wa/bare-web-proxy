package proxy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"bare-web-proxy/internal/proxy/modifiers"
)

const proxyBaseURL = "/proxy"

var styleCloseRegex = regexp.MustCompile(`(?i)</style>`)

// processHTML strips unwanted tags, injects CSS, rewrites links, and embeds the toolbar.
func (h *Handler) processHTML(rawHTML string, targetURL string, cssTexts []string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return "", err
	}

	stripTags(doc)
	injectCSS(doc, cssTexts)
	rewriteLinks(doc, targetURL)
	injectToolbar(doc, targetURL)
	modifiers.ModifyDocument(doc, targetURL)

	return doc.Html()
}

func stripTags(doc *goquery.Document) {
	doc.Find("script, noscript, iframe, img, svg, video, style, link[rel='stylesheet']").Remove()
}

func injectCSS(doc *goquery.Document, cssTexts []string) {
	for _, cssText := range cssTexts {
		if cssText == "" {
			continue
		}
		// XSS対策: CSS内の </style> をエスケープ
		safe := styleCloseRegex.ReplaceAllString(cssText, `/* style closed */`)
		doc.Find("head").AppendHtml(`<style data-proxy-style="original">` + "\n" + safe + "\n</style>")
	}
	doc.Find("head").AppendHtml(`<link rel="stylesheet" id="proxy-reader-style" href="/proxy/assets/reader.css">`)
}

func rewriteLinks(doc *goquery.Document, targetURL string) {
	base, _ := url.Parse(targetURL)

	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		absolute := resolveHref(href, base)
		if absolute == "" {
			return
		}
		s.SetAttr("href", fmt.Sprintf("%s?url=%s", proxyBaseURL, url.QueryEscape(absolute)))
	})
}

func resolveHref(href string, base *url.URL) string {
	switch {
	case strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://"):
		return href
	case strings.HasPrefix(href, "//"):
		scheme := "https"
		if base != nil && base.Scheme != "" {
			scheme = base.Scheme
		}
		return scheme + ":" + href
	default:
		if base == nil {
			return ""
		}
		u, err := url.Parse(href)
		if err != nil {
			return ""
		}
		return base.ResolveReference(u).String()
	}
}

func injectToolbar(doc *goquery.Document, targetURL string) {
	doc.Find("body").PrependHtml(`<div id="proxy-toolbar-container"></div>`)

	jsURL, err := json.Marshal(targetURL)
	if err != nil {
		jsURL = []byte(`""`)
	}
	doc.Find("body").AppendHtml(fmt.Sprintf(`<script>window.__PROXY_TARGET_URL__ = %s;</script>`, jsURL))
	doc.Find("body").AppendHtml(`<script src="/proxy/assets/toolbar.js"></script>`)
}
