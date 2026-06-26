package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/chromedp/chromedp"
	"bare-web-proxy/internal/proxy"
)

func main() {
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

	log.Printf("Go Proxy Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
