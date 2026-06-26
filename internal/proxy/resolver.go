package proxy

import (
	"net/url"
	"strings"
)

// resolveTargetURL converts queryVal to a valid proxy destination URL.
// It detects whether queryVal is a URL/domain or a search query.
func resolveTargetURL(queryVal string) string {
	queryVal = strings.TrimSpace(queryVal)
	if queryVal == "" {
		return ""
	}

	// すでに http:// または https:// で始まっているか
	lowerVal := strings.ToLower(queryVal)
	if strings.HasPrefix(lowerVal, "http://") || strings.HasPrefix(lowerVal, "https://") {
		_, err := url.ParseRequestURI(queryVal)
		if err == nil {
			return queryVal
		}
	}

	// スキームがない場合、ドメイン/ホスト名らしいか判定
	// スペースを含まず、かつ「ドットを含む」か「コロンを含む」か「スラッシュを含む」か「localhostである」場合はURLとみなす
	hasSpace := strings.ContainsAny(queryVal, " \t\n\r")
	isDomain := false
	if !hasSpace {
		if strings.Contains(queryVal, ".") ||
			strings.Contains(queryVal, ":") ||
			strings.Contains(queryVal, "/") ||
			queryVal == "localhost" {
			isDomain = true
		}
	}

	if isDomain {
		target := "http://" + queryVal
		_, err := url.ParseRequestURI(target)
		if err == nil {
			return target
		}
	}

	// それ以外はDuckDuckGo検索クエリとする
	return "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(queryVal)
}
