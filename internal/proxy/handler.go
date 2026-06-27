package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

var concurrentSemaphore = make(chan struct{}, 5)

// Handler handles HTTP requests for the proxy.
type Handler struct {
	allocatorContext context.Context
	assetMap         map[string]assetEntry
}

// NewHandler creates a new Handler with the given remote allocator context.
func NewHandler(allocatorContext context.Context) *Handler {
	return &Handler{
		allocatorContext: allocatorContext,
		assetMap:         buildAssetMap(),
	}
}

// HandleProxy renders a target page via headless Chrome and returns processed HTML.
func (h *Handler) HandleProxy(w http.ResponseWriter, r *http.Request) {
	targetURL, err := parseTargetURL(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	select {
	case concurrentSemaphore <- struct{}{}:
		defer func() { <-concurrentSemaphore }()
	case <-r.Context().Done():
		http.Error(w, "server busy", http.StatusServiceUnavailable)
		return
	}

	startTime := time.Now()

	ctx, ctxCancel := chromedp.NewContext(h.allocatorContext)
	defer ctxCancel()
	ctx, timeCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeCancel()

	userAgent := r.UserAgent()
	if userAgent == "" {
		userAgent = defaultUserAgent
	}

	rawHTML, cssTexts, totalNetworkBytes, err := renderPage(ctx, targetURL, userAgent)
	if err != nil {
		slog.Error("chrome render failed", slog.String("url", targetURL), slog.Any("error", err))
		http.Error(w, fmt.Sprintf("render error: %v", err), http.StatusInternalServerError)
		return
	}

	processedHTML, err := h.processHTML(rawHTML, targetURL, cssTexts)
	if err != nil {
		slog.Error("html processing failed", slog.String("url", targetURL), slog.Any("error", err))
		http.Error(w, "html processing error", http.StatusInternalServerError)
		return
	}

	finalSize, isGzip, err := compressAndWrite(w, r, processedHTML)
	if err != nil {
		slog.Error("write failed", slog.String("url", targetURL), slog.Any("error", err))
		return
	}

	origSize := int(totalNetworkBytes)
	if origSize == 0 {
		origSize = len(rawHTML)
	}
	go logCompression(targetURL, origSize, finalSize, isGzip, startTime)
}

func parseTargetURL(r *http.Request) (string, error) {
	if u := r.URL.Query().Get("url"); u != "" {
		return u, nil
	}
	if q := r.URL.Query().Get("q"); q != "" {
		return resolveTargetURL(q), nil
	}
	return "", fmt.Errorf("'url' or 'q' parameter is required")
}

func compressAndWrite(w http.ResponseWriter, r *http.Request, html string) (int, bool, error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		b := []byte(html)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(b)
		return len(b), false, err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(html)); err != nil {
		gz.Close()
		return 0, false, err
	}
	if err := gz.Close(); err != nil {
		return 0, false, err
	}

	w.Header().Set("Content-Encoding", "gzip")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(buf.Bytes())
	return buf.Len(), true, err
}

func logCompression(urlStr string, origSize, compSize int, isGzip bool, startTime time.Time) {
	saved := origSize - compSize
	attrs := []slog.Attr{
		slog.String("url", urlStr),
		slog.Float64("original_kb", float64(origSize)/1024),
		slog.Float64("compressed_kb", float64(compSize)/1024),
		slog.Float64("saved_kb", float64(saved)/1024),
		slog.Bool("gzip", isGzip),
		slog.Float64("duration_ms", float64(time.Since(startTime).Milliseconds())),
	}
	if origSize > 0 && saved > 0 {
		attrs = append(attrs, slog.Float64("reduction_percent", float64(saved)/float64(origSize)*100))
	}
	slog.LogAttrs(context.Background(), slog.LevelInfo, "compression_success", attrs...)
}
