package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

var (
	proxyBaseURL        string
	styleCloseRegex     = regexp.MustCompile(`(?i)</style>`)
	concurrentSemaphore = make(chan struct{}, 5) // 最大同時リクエスト数を5に制限
)

//go:embed frontend/index.html
var frontendHTML string

//go:embed frontend/toolbar.js
var toolbarJS string

//go:embed frontend/reader.css
var readerCSS string
func main() {
	// 環境変数から設定を取得
	chromeURL := os.Getenv("CHROME_URL") // 例: ws://127.0.0.1:9222
	if chromeURL == "" {
		chromeURL = "ws://127.0.0.1:9222"
	}
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	proxyBaseURL = "/proxy"

	// 外部のHeadless Chrome (サイドカー) に接続する設定
	allocatorContext, cancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer cancel()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(frontendHTML))
	})

	http.HandleFunc("/proxy", func(w http.ResponseWriter, r *http.Request) {
		targetURL := r.URL.Query().Get("url")
		if targetURL == "" {
			queryVal := r.URL.Query().Get("q")
			if queryVal == "" {
				http.Error(w, "Error: 'url' or 'q' parameter is required", http.StatusBadRequest)
				return
			}
			targetURL = resolveTargetURL(queryVal)
		}

		// 同時接続数の制御 (最大同時5リクエストに制限してChromeハングを防止)
		select {
		case concurrentSemaphore <- struct{}{}:
			defer func() { <-concurrentSemaphore }()
		case <-r.Context().Done():
			http.Error(w, "Server Busy: timeout waiting for slot", http.StatusServiceUnavailable)
			return
		}

		startTime := time.Now()

		// 1. Chrome用のコンテキスト作成
		ctx, ctxCancel := chromedp.NewContext(allocatorContext)
		defer ctxCancel()

		// クライアントのUser-Agentを取得
		userAgent := r.UserAgent()
		if userAgent == "" {
			userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
		}

		// タイムアウト設定 (30秒)
		ctx, timeCancel := context.WithTimeout(ctx, 30*time.Second)
		defer timeCancel()

		// 2. Chrome側でページをレンダリングしてHTMLとCSSを取得
		rawHTML, cssTexts, totalNetworkBytes, err := renderPage(ctx, targetURL, userAgent)
		if err != nil {
			log.Printf("Chrome Error [%s]: %v", targetURL, err)
			http.Error(w, fmt.Sprintf("Render Error: %v", err), http.StatusInternalServerError)
			return
		}

		// 3. HTMLの削ぎ落とし＆URL書き換え＆ツールバー埋め込み処理
		processedHTML, err := processHTML(rawHTML, targetURL, cssTexts)
		if err != nil {
			log.Printf("Process HTML Error [%s]: %v", targetURL, err)
			http.Error(w, "HTML Parse/Generation Error", http.StatusInternalServerError)
			return
		}

		// 4. クライアントへの返却
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(processedHTML))

		// 5. パフォーマンスログの出力 (goroutineで非同期実行してレスポンスに影響を与えない)
		originalSize := int(totalNetworkBytes)
		if originalSize == 0 {
			originalSize = len(rawHTML)
		}
		go logCompression(targetURL, originalSize, len(processedHTML), startTime)
	})

	log.Printf("Go Proxy Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// renderPage renders the page using chromedp and returns the raw HTML, CSS contents, and total network bytes transfered.
func renderPage(ctx context.Context, targetURL string, userAgent string) (string, []string, int64, error) {
	// ネットワーク転送サイズとスタイルシート情報を集計するイベントリスナーの登録
	var totalNetworkBytes int64
	var stylesheetIDs []cdp.StyleSheetID
	var mu sync.Mutex
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if e, ok := ev.(*network.EventLoadingFinished); ok {
			mu.Lock()
			totalNetworkBytes += int64(e.EncodedDataLength)
			mu.Unlock()
		}
		if e, ok := ev.(*css.EventStyleSheetAdded); ok {
			mu.Lock()
			stylesheetIDs = append(stylesheetIDs, e.Header.StyleSheetID)
			mu.Unlock()
		}
	})

	var rawHTML string

	// Chrome側でページをレンダリングしてHTMLを取得
	err := chromedp.Run(ctx,
		network.Enable(), // ネットワーク制御を有効化
		css.Enable(),     // CSSドメインを有効化
		// クライアントのUser-Agentと、Accept-Language/Platformを設定してブラウザらしく見せる
		emulation.SetUserAgentOverride(userAgent).
			WithAcceptLanguage("ja-JP,ja;q=0.9,en-US;q=0.8,en;q=0.7").
			WithPlatform("Windows"),
		// navigator.webdriver を隠蔽してボット検知を回避
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(`
				Object.defineProperty(navigator, 'webdriver', {
					get: () => undefined
				});
				window.chrome = {
					runtime: {},
					loadTimes: function() {},
					csi: function() {},
					app: {}
				};
				Object.defineProperty(navigator, 'plugins', {
					get: () => [
						{ description: "Portable Document Format", filename: "internal-pdf-viewer", name: "Chrome PDF Viewer" }
					]
				});
			`).Do(ctx)
			return err
		}),
		chromedp.Navigate(targetURL),
		// ネットワークが安定するか、Body要素が出るまで待つ
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.OuterHTML(`html`, &rawHTML),
	)

	if err != nil {
		return "", nil, 0, err
	}

	// CSSテキストの取得
	mu.Lock()
	ids := make([]cdp.StyleSheetID, len(stylesheetIDs))
	copy(ids, stylesheetIDs)
	mu.Unlock()

	cssTexts := make([]string, len(ids))
	actions := make([]chromedp.Action, len(ids))
	for i, id := range ids {
		idx := i
		targetID := id
		actions[i] = chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cssTexts[idx], err = css.GetStyleSheetText(targetID).Do(ctx)
			return err
		})
	}

	if len(actions) > 0 {
		if err := chromedp.Run(ctx, actions...); err != nil {
			log.Printf("Failed to get stylesheet texts: %v", err)
		}
	}

	return rawHTML, cssTexts, totalNetworkBytes, nil
}

// processHTML processes raw HTML (stripping tags, injection reader mode CSS and the custom toolbar, rewriting a-tag links).
func processHTML(rawHTML string, targetURL string, cssTexts []string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return "", err
	}

	// 不要なタグの排除
	doc.Find("script, noscript, iframe, img, svg, video, style, link[rel='stylesheet']").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})

	// 取得したCSSを<style>タグとして埋め込む (事前コンパイルされた正規表現を使用)
	for _, cssText := range cssTexts {
		if cssText == "" {
			continue
		}
		// XSS対策: CSS内の </style> をエスケープ
		safeCSS := styleCloseRegex.ReplaceAllString(cssText, `/* style closed */`)
		doc.Find("head").AppendHtml("<style data-proxy-style=\"original\">\n" + safeCSS + "\n</style>")
	}

	// リーダーモード用CSSの埋め込み
	doc.Find("head").AppendHtml("<style id=\"proxy-reader-style\">\n" + readerCSS + "\n</style>")

	// aタグのリンク書き換え
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		var absoluteURL string
		if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
			absoluteURL = href
		} else if strings.HasPrefix(href, "//") {
			base, err := url.Parse(targetURL)
			scheme := "https"
			if err == nil && base.Scheme != "" {
				scheme = base.Scheme
			}
			absoluteURL = scheme + ":" + href
		} else {
			// 相対パスを絶対URLに変換
			base, err := url.Parse(targetURL)
			if err != nil {
				return
			}
			u, err := url.Parse(href)
			if err != nil {
				return
			}
			absoluteURL = base.ResolveReference(u).String()
		}

		// 中継サーバー経由に書き換え
		newHref := fmt.Sprintf("%s?url=%s", proxyBaseURL, url.QueryEscape(absoluteURL))
		s.SetAttr("href", newHref)
	})

	// ツールバーの埋め込み
	doc.Find("body").PrependHtml("<div id=\"proxy-toolbar-container\"></div>")
	jsTargetURL, err := json.Marshal(targetURL)
	if err != nil {
		jsTargetURL = []byte(`""`)
	}
	doc.Find("body").AppendHtml(fmt.Sprintf("<script>window.__PROXY_TARGET_URL__ = %s;</script>", string(jsTargetURL)))
	doc.Find("body").AppendHtml("<script>\n" + toolbarJS + "\n</script>")

	return doc.Html()
}

// logCompression handles asynchronous compression ratio logging.
func logCompression(urlStr string, origSize, compSize int, startTime time.Time) {
	savedBytes := origSize - compSize
	reductionRate := 0.0
	if origSize > 0 {
		reductionRate = (float64(savedBytes) / float64(origSize)) * 100
	}
	log.Printf("[SUCCESS] URL: %s | Original: %.2f KB | Compressed: %.2f KB | Saved: %.2f KB (削減率: %.1f%%) | Time: %v",
		urlStr,
		float64(origSize)/1024.0,
		float64(compSize)/1024.0,
		float64(savedBytes)/1024.0,
		reductionRate,
		time.Since(startTime),
	)
}

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
