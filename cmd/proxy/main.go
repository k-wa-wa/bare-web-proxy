package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/chromedp/chromedp"
	"bare-web-proxy/internal/proxy"
)

func main() {
	// デフォルトロガーとしてJSONハンドラーを設定
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	chromeURL := os.Getenv("CHROME_URL")
	if chromeURL == "" {
		chromeURL = "ws://127.0.0.1:9222"
	}
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	allocatorContext, cancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer cancel()

	h := proxy.NewHandler(allocatorContext)

	http.HandleFunc("/", h.HandleRoot)
	http.HandleFunc("/proxy", h.HandleProxy)
	http.HandleFunc("/proxy/assets/", h.HandleAssets)

	slog.Info("Go Proxy Server starting", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		slog.Error("Server failed to start", slog.Any("error", err))
		os.Exit(1)
	}
}
