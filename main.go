package main

import (
	"context"
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

var proxyBaseURL string

const frontendHTML = `<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Bare Web Proxy</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;800&family=Noto+Sans+JP:wght@300;400;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --primary: #6366f1;
            --primary-hover: #4f46e5;
            --bg-gradient: linear-gradient(135deg, #0f172a 0%, #1e1b4b 100%);
            --glass-bg: rgba(255, 255, 255, 0.04);
            --glass-border: rgba(255, 255, 255, 0.08);
            --text-main: #f8fafc;
            --text-muted: #94a3b8;
        }

        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }

        body {
            font-family: 'Outfit', 'Noto Sans JP', sans-serif;
            background: var(--bg-gradient);
            color: var(--text-main);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
            overflow: hidden;
            position: relative;
        }

        /* 綺麗なグラデーションサークル背景 */
        body::before, body::after {
            content: '';
            position: absolute;
            width: 300px;
            height: 300px;
            border-radius: 50%;
            background: rgba(99, 102, 241, 0.15);
            filter: blur(80px);
            z-index: 0;
        }
        body::before {
            top: 15%;
            left: 15%;
        }
        body::after {
            bottom: 15%;
            right: 15%;
        }

        .container {
            position: relative;
            z-index: 10;
            width: 100%;
            max-width: 540px;
            background: var(--glass-bg);
            border: 1px solid var(--glass-border);
            border-radius: 24px;
            padding: 40px 30px;
            backdrop-filter: blur(20px);
            -webkit-backdrop-filter: blur(20px);
            box-shadow: 0 20px 50px rgba(0, 0, 0, 0.3);
            text-align: center;
            animation: fadeIn 0.8s ease-out;
        }

        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(20px); }
            to { opacity: 1; transform: translateY(0); }
        }

        .logo-area {
            margin-bottom: 30px;
        }

        .logo-title {
            font-size: 2.5rem;
            font-weight: 800;
            letter-spacing: -0.05em;
            background: linear-gradient(to right, #a5b4fc, #6366f1);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            margin-bottom: 10px;
        }

        .logo-subtitle {
            font-size: 0.95rem;
            color: var(--text-muted);
            line-height: 1.6;
            font-weight: 300;
        }

        .form-group {
            display: flex;
            flex-direction: column;
            gap: 12px;
            margin-bottom: 25px;
        }

        .input-wrapper {
            position: relative;
            display: flex;
            align-items: center;
        }

        .url-input {
            width: 100%;
            padding: 16px 20px;
            border-radius: 14px;
            border: 1px solid var(--glass-border);
            background: rgba(0, 0, 0, 0.2);
            color: var(--text-main);
            font-size: 1rem;
            font-family: inherit;
            outline: none;
            transition: all 0.3s ease;
        }

        .url-input:focus {
            border-color: var(--primary);
            box-shadow: 0 0 0 4px rgba(99, 102, 241, 0.2);
            background: rgba(0, 0, 0, 0.3);
        }

        .submit-btn {
            padding: 16px 24px;
            border: none;
            border-radius: 14px;
            background: var(--primary);
            color: white;
            font-size: 1rem;
            font-weight: 600;
            font-family: inherit;
            cursor: pointer;
            transition: all 0.3s ease;
            box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
        }

        .submit-btn:hover {
            background: var(--primary-hover);
            transform: translateY(-2px);
            box-shadow: 0 6px 20px rgba(99, 102, 241, 0.4);
        }

        .submit-btn:active {
            transform: translateY(0);
        }

        .test-links {
            margin-top: 15px;
            padding-top: 20px;
            border-top: 1px solid var(--glass-border);
            display: flex;
            flex-direction: column;
            gap: 10px;
            align-items: center;
        }

        .test-title {
            font-size: 0.8rem;
            color: var(--text-muted);
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        .test-link {
            color: #a5b4fc;
            text-decoration: none;
            font-size: 0.9rem;
            transition: color 0.2s ease;
            display: inline-flex;
            align-items: center;
            gap: 6px;
        }

        .test-link:hover {
            color: var(--text-main);
            text-decoration: underline;
        }

        .error-message {
            color: #f87171;
            font-size: 0.85rem;
            min-height: 20px;
            transition: opacity 0.2s ease;
            opacity: 0;
        }

        .error-message.show {
            opacity: 1;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo-area">
            <h1 class="logo-title">Bare Web Proxy</h1>
            <p class="logo-subtitle">Go & Headless Chrome サイドカーによる、通信容量を極限まで削減する超軽量Webプロキシ。</p>
        </div>

        <form id="proxyForm" onsubmit="handleSubmit(event)">
            <div class="form-group">
                <div class="input-wrapper">
                    <input type="text" id="urlInput" class="url-input" placeholder="https://example.com" required autocomplete="off">
                </div>
                <div id="errorArea" class="error-message"></div>
                <button type="submit" class="submit-btn">Go Proxy</button>
            </div>
        </form>

        <div class="test-links">
            <span class="test-title">ローカル疎通検証用リンク</span>
            <a href="/proxy?url=http%3A%2F%2F127.0.0.1%3A3000%2Fdummy" class="test-link">
                📄 ダミーのテストページ (/dummy) を経由して表示
            </a>
        </div>
    </div>

    <script>
        function handleSubmit(event) {
            event.preventDefault();
            const input = document.getElementById('urlInput');
            const errorArea = document.getElementById('errorArea');
            let urlVal = input.value.trim();

            if (!urlVal) return;

            let targetURL = urlVal;

            // URLかどうかの簡易判定
            if (/^https?:\/\//i.test(urlVal)) {
                // スキームがすでに明示されている場合
                try {
                    new URL(urlVal);
                } catch (e) {
                    errorArea.textContent = '有効なURLを入力してください（例: https://example.com）';
                    errorArea.classList.add('show');
                    return;
                }
            } else {
                // スキームがない場合、ドメイン/ホスト名らしいか判定
                // スペースを含まず、かつ「ドットを含む」か「localhost」である場合はURLとみなす
                const isDomain = !/\s/.test(urlVal) && (urlVal.includes('.') || urlVal.startsWith('localhost'));
                if (isDomain) {
                    targetURL = 'http://' + urlVal;
                    try {
                        new URL(targetURL);
                    } catch (e) {
                        // URLとして無効な場合はDuckDuckGo検索クエリとする
                        targetURL = 'https://html.duckduckgo.com/html/?q=' + encodeURIComponent(urlVal);
                    }
                } else {
                    // それ以外はDuckDuckGo検索クエリとする
                    targetURL = 'https://html.duckduckgo.com/html/?q=' + encodeURIComponent(urlVal);
                }
            }

            errorArea.classList.remove('show');
            // プロキシURLへリダイレクト
            window.location.href = '/proxy?url=' + encodeURIComponent(targetURL);
        }
    </script>
</body>
</html>`

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
			http.Error(w, "Error: 'url' parameter is required", http.StatusBadRequest)
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

		// 2. Chrome側でページをレンダリングしてHTMLを取得
		err := chromedp.Run(ctx,
			network.Enable(), // ネットワーク制御を有効化
			css.Enable(),     // CSSドメインを有効化
			// クライアントのUser-Agentと、Accept-Language/Platformを設定してブラウザらしく見せる
			emulation.SetUserAgentOverride(userAgent).
				WithAcceptLanguage("ja-JP,ja;q=0.9,en-US;q=0.8,en;q=0.7").
				WithPlatform("Windows"),
			// navigator.webdriver を隠蔽してボット検知を回避 (さらにchrome/pluginsオブジェクトもモック)
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
			log.Printf("Chrome Error [%s]: %v", targetURL, err)
			http.Error(w, fmt.Sprintf("Render Error: %v", err), http.StatusInternalServerError)
			return
		}

		// CSSテキストの取得 (1回の chromedp.Run で一括取得して最適化)
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

		originalSize := int(totalNetworkBytes)
		if originalSize == 0 {
			originalSize = len(rawHTML)
		}

		// 3. HTMLの削ぎ落とし＆URL書き換え処理
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
		if err != nil {
			http.Error(w, "HTML Parse Error", http.StatusInternalServerError)
			return
		}

		// 不要なタグの排除
		doc.Find("script, noscript, iframe, img, svg, video, style, link[rel='stylesheet']").Each(func(i int, s *goquery.Selection) {
			s.Remove()
		})

		// 取得したCSSを<style>タグとして埋め込む
		styleCloseRegex := regexp.MustCompile(`(?i)</style>`)
		for _, cssText := range cssTexts {
			if cssText == "" {
				continue
			}
			// XSS対策: CSS内の </style> をエスケープ
			safeCSS := styleCloseRegex.ReplaceAllString(cssText, `/* style closed */`)
			doc.Find("head").AppendHtml("<style>\n" + safeCSS + "\n</style>")
		}

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
			absoluteURL = resolveRedirectURL(absoluteURL)

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

// resolveRedirectURL extracts the final target URL from search engine redirect links (e.g. DuckDuckGo or Google).
func resolveRedirectURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// DuckDuckGo redirect format: https://duckduckgo.com/l/?uddg=...
	if (u.Host == "duckduckgo.com" || u.Host == "html.duckduckgo.com") && (u.Path == "/l/" || u.Path == "/l") {
		uddg := u.Query().Get("uddg")
		if uddg != "" {
			return uddg
		}
	}

	// Google redirect format: https://www.google.com/url?q=...
	if strings.Contains(u.Host, "google.") && u.Path == "/url" {
		q := u.Query().Get("q")
		if q != "" {
			return q
		}
	}

	return rawURL
}
