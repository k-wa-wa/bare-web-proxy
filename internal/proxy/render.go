package proxy

import (
	"context"
	"log/slog"
	"sync"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

var blockedURLPatterns = []*network.BlockPattern{
	// 画像
	{URLPattern: "*://*:*/*.png", Block: true},
	{URLPattern: "*://*:*/*.jpg", Block: true},
	{URLPattern: "*://*:*/*.jpeg", Block: true},
	{URLPattern: "*://*:*/*.gif", Block: true},
	{URLPattern: "*://*:*/*.webp", Block: true},
	{URLPattern: "*://*:*/*.svg", Block: true},
	{URLPattern: "*://*:*/*.ico", Block: true},
	// 動画・音声
	{URLPattern: "*://*:*/*.mp4", Block: true},
	{URLPattern: "*://*:*/*.webm", Block: true},
	{URLPattern: "*://*:*/*.m3u8", Block: true},
	{URLPattern: "*://*:*/*.mp3", Block: true},
	{URLPattern: "*://*:*/*.ogg", Block: true},
	{URLPattern: "*://*:*/*.wav", Block: true},
	{URLPattern: "*://*:*/*.ts", Block: true},
	// フォント
	{URLPattern: "*://*:*/*.woff", Block: true},
	{URLPattern: "*://*:*/*.woff2", Block: true},
	{URLPattern: "*://*:*/*.ttf", Block: true},
	{URLPattern: "*://*:*/*.otf", Block: true},
	// 広告・アナリティクス・トラッカー
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
}

// renderPage renders the page using chromedp and returns the raw HTML, CSS contents, and total network bytes transferred.
func renderPage(ctx context.Context, targetURL string, userAgent string) (string, []string, int64, error) {
	var totalNetworkBytes int64
	var stylesheetIDs []cdp.StyleSheetID
	var mu sync.Mutex

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventLoadingFinished:
			mu.Lock()
			totalNetworkBytes += int64(e.EncodedDataLength)
			mu.Unlock()
		case *css.EventStyleSheetAdded:
			mu.Lock()
			stylesheetIDs = append(stylesheetIDs, e.Header.StyleSheetID)
			mu.Unlock()
		}
	})

	var rawHTML string
	err := chromedp.Run(ctx,
		network.Enable(),
		network.SetBlockedURLs().WithURLPatterns(blockedURLPatterns),
		css.Enable(),
		emulation.SetUserAgentOverride(userAgent).
			WithAcceptLanguage("ja-JP,ja;q=0.9,en-US;q=0.8,en;q=0.7").
			WithPlatform("Windows"),
		hideWebDriver(),
		chromedp.Navigate(targetURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.OuterHTML(`html`, &rawHTML),
	)
	if err != nil {
		return "", nil, 0, err
	}

	cssTexts := fetchStylesheets(ctx, stylesheetIDs, &mu)
	return rawHTML, cssTexts, totalNetworkBytes, nil
}

// hideWebDriver injects JS to conceal automation markers from bot detection.
func hideWebDriver() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(`
			Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
			window.chrome = { runtime: {}, loadTimes: function() {}, csi: function() {}, app: {} };
			Object.defineProperty(navigator, 'plugins', {
				get: () => [{ description: "Portable Document Format", filename: "internal-pdf-viewer", name: "Chrome PDF Viewer" }]
			});
		`).Do(ctx)
		return err
	})
}

// fetchStylesheets retrieves CSS text for each collected stylesheet ID.
func fetchStylesheets(ctx context.Context, ids []cdp.StyleSheetID, mu *sync.Mutex) []string {
	mu.Lock()
	snapshot := make([]cdp.StyleSheetID, len(ids))
	copy(snapshot, ids)
	mu.Unlock()

	texts := make([]string, len(snapshot))
	actions := make([]chromedp.Action, len(snapshot))
	for i, id := range snapshot {
		i, id := i, id
		actions[i] = chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			texts[i], err = css.GetStyleSheetText(id).Do(ctx)
			if err != nil {
				slog.Debug("failed to get stylesheet", slog.String("id", string(id)), slog.Any("error", err))
			}
			return nil
		})
	}
	if len(actions) > 0 {
		if err := chromedp.Run(ctx, actions...); err != nil {
			slog.Warn("stylesheet retrieval incomplete", slog.Any("error", err))
		}
	}
	return texts
}
