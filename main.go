package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

var proxyBaseURL string

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
	proxyBaseURL = fmt.Sprintf("http://localhost:%s/proxy", port)

	// 外部のHeadless Chrome (サイドカー) に接続する設定
	allocatorContext, cancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer cancel()

	http.HandleFunc("/proxy", func(w http.ResponseWriter, r *http.Request) {
		targetURL := r.URL.Query().Get("url")
		if targetURL == "" {
			http.Error(w, "Error: 'url' parameter is required", http.StatusBadRequest)
			return
		}

		startTime := time.Now()

		// 1. Chrome用のコンテキスト作成
		ctx, ctxCancel := chromedp.NewContext(allocatorContext)
		defer ctxCancel()

		// タイムアウト設定 (30秒)
		ctx, timeCancel := context.WithTimeout(ctx, 30*time.Second)
		defer timeCancel()

		var rawHTML string
		// 2. Chrome側でページをレンダリングしてHTMLを取得
		err := chromedp.Run(ctx,
			chromedp.Navigate(targetURL),
			// ネットワークが安定するか、Body要素が出るまで待つ
			chromedp.WaitVisible(`body`, chromedp.ByQuery),
			chromedp.OuterHTML(`html`, &rawHTML),
		)

		if err != nil {
			log.Printf("Chrome Error [%s]: %v", targetURL, err)
			http.Error(w, fmt.Sprintf("Render Error: %v", err), http.StatusInternalServerError)
			return
		}

		originalSize := len(rawHTML)

		// 3. HTMLの削ぎ落とし＆URL書き換え処理
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
		if err != nil {
			http.Error(w, "HTML Parse Error", http.StatusInternalServerError)
			return
		}

		// 不要なタグの排除
		doc.Find("script, noscript, iframe, img, video, style, link[rel='stylesheet']").Each(func(i int, s *goquery.Selection) {
			s.Remove()
		})

		// aタグのリンク書き換え
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists {
				return
			}

			// 相対パスを絶対URLに変換
			base, _ := url.Parse(targetURL)
			u, err := url.Parse(href)
			if err != nil {
				return
			}
			absoluteURL := base.ResolveReference(u).String()

			// 中継サーバー経由に書き換え
			newHref := fmt.Sprintf("%s?url=%s", proxyBaseURL, url.QueryEscape(absoluteURL))
			s.SetAttr("href", newHref)
		})

		// 最終的なHTML文字列の生成
		processedHTML, err := doc.Html()
		if err != nil {
			http.Error(w, "HTML Generation Error", http.StatusInternalServerError)
			return
		}

		compressedSize := len(processedHTML)
		savedBytes := originalSize - compressedSize
		reductionRate := 0.0
		if originalSize > 0 {
			reductionRate = (float64(savedBytes) / float64(originalSize)) * 100
		}

		// 4. パフォーマンスログの出力 (URLごとにどれだけ削減できたか)
		log.Printf("[SUCCESS] URL: %s | Original: %.2f KB | Compressed: %.2f KB | Saved: %.2f KB (削減率: %.1f%%) | Time: %v",
			targetURL,
			float64(originalSize)/1024.0,
			float64(compressedSize)/1024.0,
			float64(savedBytes)/1024.0,
			reductionRate,
			time.Since(startTime),
		)

		// 5. スマホへの返却
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(processedHTML))
	})

	http.HandleFunc("/dummy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Dummy Test Page</title>
    <style>
        body { font-family: sans-serif; }
        .red { color: red; }
    </style>
    <script>
        console.log("This script should be removed");
    </script>
</head>
<body>
    <h1>Hello, World!</h1>
    <p class="red">This is a dummy page for testing proxy compression.</p>
    <img src="dummy.png" alt="This image should be removed" />
    <a href="https://example.com/another-page">Link to another page</a>
    <a href="/dummy?relative=1">Relative Link</a>
</body>
</html>`))
	})

	log.Printf("Go Proxy Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
