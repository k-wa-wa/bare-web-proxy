package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
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

	"bare-web-proxy/internal/proxy/modifiers"
)

var (
	proxyBaseURL        = "/proxy"
	styleCloseRegex     = regexp.MustCompile(`(?i)</style>`)
	concurrentSemaphore = make(chan struct{}, 5) // 最大同時リクエスト数を5に制限
)

//go:embed static
var staticFS embed.FS

type assetEntry struct {
	content     []byte
	contentType string
	etag        string
}

// Handler handles HTTP requests for root and proxy endpoints
type Handler struct {
	allocatorContext context.Context
	assetMap         map[string]assetEntry
}

// NewHandler creates a new Handler with the given remote allocator context
func NewHandler(allocatorContext context.Context) *Handler {
	return &Handler{
		allocatorContext: allocatorContext,
		assetMap:         buildAssetMap(),
	}
}

func buildAssetMap() map[string]assetEntry {
	extTypes := map[string]string{
		".js":   "application/javascript; charset=utf-8",
		".css":  "text/css; charset=utf-8",
		".html": "text/html; charset=utf-8",
	}

	dirEntries, err := staticFS.ReadDir("static")
	if err != nil {
		panic(fmt.Sprintf("failed to read embedded static: %v", err))
	}

	m := make(map[string]assetEntry, len(dirEntries))
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		ct, ok := extTypes[path.Ext(name)]
		if !ok {
			continue
		}
		content, err := staticFS.ReadFile("static/" + name)
		if err != nil {
			panic(fmt.Sprintf("embedded asset not found: %s: %v", name, err))
		}
		hash := sha256.Sum256(content)
		m[name] = assetEntry{
			content:     content,
			contentType: ct,
			etag:        hex.EncodeToString(hash[:])[:8],
		}
	}
	return m
}

// HandleAssets serves static assets like toolbar.js and reader.css with aggressive caching
func (h *Handler) HandleAssets(w http.ResponseWriter, r *http.Request) {
	filename := path.Base(r.URL.Path)
	entry, ok := h.assetMap[filename]
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("ETag", `"`+entry.etag+`"`)

	// If-None-Match による検証（304 返却）
	if r.Header.Get("If-None-Match") == `"`+entry.etag+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// キャッシュの再検証（If-None-Match）を毎回強制する
	w.Header().Set("Cache-Control", "no-cache")

	w.Header().Set("Content-Type", entry.contentType)
	w.WriteHeader(http.StatusOK)
	w.Write(entry.content)
}

// HandleRoot serves the main proxy UI page
func (h *Handler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	entry := h.assetMap["index.html"]
	w.Header().Set("Content-Type", entry.contentType)
	w.Write(entry.content)
}

// HandleProxy handles proxy request, rendering pages using headless Chrome and rewriting HTML
func (h *Handler) HandleProxy(w http.ResponseWriter, r *http.Request) {
	targetURL, err := parseTargetURL(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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
	ctx, ctxCancel := chromedp.NewContext(h.allocatorContext)
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
		slog.Error("Chrome error", slog.String("url", targetURL), slog.Any("error", err))
		http.Error(w, fmt.Sprintf("Render Error: %v", err), http.StatusInternalServerError)
		return
	}

	// 3. HTMLの削ぎ落とし＆URL書き換え＆ツールバー埋め込み処理
	processedHTML, err := h.processHTML(rawHTML, targetURL, cssTexts)
	if err != nil {
		slog.Error("Process HTML error", slog.String("url", targetURL), slog.Any("error", err))
		http.Error(w, "HTML Parse/Generation Error", http.StatusInternalServerError)
		return
	}

	// 4. クライアントへの返却 (Gzip圧縮判定と圧縮処理)
	finalSize, isGzip, err := compressAndWrite(w, r, processedHTML)
	if err != nil {
		slog.Error("Compression/Write error", slog.String("url", targetURL), slog.Any("error", err))
		http.Error(w, "Response Generation Error", http.StatusInternalServerError)
		return
	}

	// 5. パフォーマンスログの出力 (goroutineで非同期実行してレスポンスに影響を与えない)
	originalSize := int(totalNetworkBytes)
	if originalSize == 0 {
		originalSize = len(rawHTML)
	}
	go logCompression(targetURL, originalSize, finalSize, isGzip, startTime)
}

// parseTargetURL extracts and resolves the destination target URL from request query parameters.
func parseTargetURL(r *http.Request) (string, error) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		queryVal := r.URL.Query().Get("q")
		if queryVal == "" {
			return "", fmt.Errorf("Error: 'url' or 'q' parameter is required")
		}
		targetURL = resolveTargetURL(queryVal)
	}
	return targetURL, nil
}

// compressAndWrite compresses processedHTML using gzip if client supports it, writes response, and returns compressed size and status.
func compressAndWrite(w http.ResponseWriter, r *http.Request, processedHTML string) (int, bool, error) {
	var responseBytes []byte
	var finalSize int
	isGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")

	if isGzip {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write([]byte(processedHTML)); err != nil {
			gz.Close()
			return 0, false, err
		}
		if err := gz.Close(); err != nil {
			return 0, false, err
		}
		responseBytes = buf.Bytes()
		finalSize = len(responseBytes)
		w.Header().Set("Content-Encoding", "gzip")
	} else {
		responseBytes = []byte(processedHTML)
		finalSize = len(responseBytes)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(responseBytes); err != nil {
		return 0, false, err
	}

	return finalSize, isGzip, nil
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
		network.SetBlockedURLs().WithURLPatterns([]*network.BlockPattern{
			// 画像アセット
			{URLPattern: "*://*:*/*.png", Block: true},
			{URLPattern: "*://*:*/*.jpg", Block: true},
			{URLPattern: "*://*:*/*.jpeg", Block: true},
			{URLPattern: "*://*:*/*.gif", Block: true},
			{URLPattern: "*://*:*/*.webp", Block: true},
			{URLPattern: "*://*:*/*.svg", Block: true},
			{URLPattern: "*://*:*/*.ico", Block: true},
			// 動画・音声アセット
			{URLPattern: "*://*:*/*.mp4", Block: true},
			{URLPattern: "*://*:*/*.webm", Block: true},
			{URLPattern: "*://*:*/*.m3u8", Block: true},
			{URLPattern: "*://*:*/*.mp3", Block: true},
			{URLPattern: "*://*:*/*.ogg", Block: true},
			{URLPattern: "*://*:*/*.wav", Block: true},
			{URLPattern: "*://*:*/*.ts", Block: true},
			// フォントアセット
			{URLPattern: "*://*:*/*.woff", Block: true},
			{URLPattern: "*://*:*/*.woff2", Block: true},
			{URLPattern: "*://*:*/*.ttf", Block: true},
			{URLPattern: "*://*:*/*.otf", Block: true},
			// 広告・アナリティクス・トラッカー関連
			{URLPattern: "*://*.google-analytics.com/*", Block: true},
			{URLPattern: "*://*.googlesyndication.com/*", Block: true},
			{URLPattern: "*://*.doubleclick.net/*", Block: true},
			{URLPattern: "*://*analytics*/*", Block: true},
			{URLPattern: "*://*adservice*/*", Block: true},
			{URLPattern: "*://*adsystem*/*", Block: true},
			{URLPattern: "*://*adnxs*/*", Block: true},
			{URLPattern: "*://*.scorecardresearch.com/*", Block: true},
			{URLPattern: "*://*.criteo.com/*", Block: true},
			{URLPattern: "*://*.hotjar.com/*", Block: true},
			{URLPattern: "*://*.outbrain.com/*", Block: true},
			{URLPattern: "*://*.taboola.com/*", Block: true},
		}),
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
			if err != nil {
				// スタイルシートが既に存在しない場合やアクセスできない場合でも、
				// 他のスタイルシートの取得を継続するために、エラーを記録してnilを返します。
				slog.Debug("Failed to get individual stylesheet text", slog.String("stylesheet_id", string(targetID)), slog.Any("error", err))
				return nil
			}
			return nil
		})
	}

	if len(actions) > 0 {
		// すべてのアクションは個別でエラーをキャッチしてnilを返すため、
		// chromedp.Run は全体のエラーで中断することなく実行されます。
		if err := chromedp.Run(ctx, actions...); err != nil {
			slog.Warn("Failed to execute stylesheet retrieval actions", slog.Any("error", err))
		}
	}

	return rawHTML, cssTexts, totalNetworkBytes, nil
}

// processHTML processes raw HTML (stripping tags, injecting references to reader mode CSS and the custom toolbar, rewriting a-tag links).
func (h *Handler) processHTML(rawHTML string, targetURL string, cssTexts []string) (string, error) {
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

	// リーダーモード用CSSの読み込み (外部ファイル参照)
	doc.Find("head").AppendHtml("<link rel=\"stylesheet\" id=\"proxy-reader-style\" href=\"/proxy/assets/reader.css\">")

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

	// ツールバーの埋め込み (外部ファイル参照)
	doc.Find("body").PrependHtml("<div id=\"proxy-toolbar-container\"></div>")
	jsTargetURL, err := json.Marshal(targetURL)
	if err != nil {
		jsTargetURL = []byte(`""`)
	}
	doc.Find("body").AppendHtml(fmt.Sprintf("<script>window.__PROXY_TARGET_URL__ = %s;</script>", string(jsTargetURL)))
	doc.Find("body").AppendHtml("<script src=\"/proxy/assets/toolbar.js\"></script>")

	// ドメイン固有の修正を適用
	modifiers.ModifyDocument(doc, targetURL)

	return doc.Html()
}

// logCompression handles asynchronous compression ratio logging.
func logCompression(urlStr string, origSize, compSize int, isGzip bool, startTime time.Time) {
	savedBytes := origSize - compSize
	cacheHit := origSize <= compSize

	attrs := []slog.Attr{
		slog.String("url", urlStr),
		slog.Float64("original_kb", float64(origSize)/1024.0),
		slog.Float64("compressed_kb", float64(compSize)/1024.0),
		slog.Float64("saved_kb", float64(savedBytes)/1024.0),
		slog.Bool("gzip", isGzip),
		slog.Float64("duration_ms", float64(time.Since(startTime).Milliseconds())),
		slog.Bool("cache_hit", cacheHit),
	}

	if !cacheHit && origSize > 0 {
		reductionRate := (float64(savedBytes) / float64(origSize)) * 100
		attrs = append(attrs, slog.Float64("reduction_rate_percent", reductionRate))
	}

	slog.LogAttrs(context.Background(), slog.LevelInfo, "compression_success", attrs...)
}
