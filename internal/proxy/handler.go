package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"

var concurrentSemaphore = make(chan struct{}, 5)

// Machine-readable failure reasons returned in error responses. Programmatic callers
// should branch on these rather than parsing the human-readable "error" message.
const (
	reasonInvalidURL       = "invalid_url"
	reasonOverloaded       = "overloaded"
	reasonRenderTimeout    = "render_timeout"
	reasonRenderBlocked    = "render_blocked"
	reasonRenderFailed     = "render_failed"
	reasonProcessingFailed = "processing_failed"
)

// errorResponse is the structured JSON body returned for any failed /proxy request.
type errorResponse struct {
	Error  string `json:"error"`
	Reason string `json:"reason"`
	Status int    `json:"status,omitempty"`
}

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
//
// By default the response is browser-oriented HTML with a toolbar injected. Callers that
// send the request header "X-Program-Mode: true" — intended for AI agents that fetch,
// summarize, or extract information from the content rather than render it for a human —
// receive the same content-bearing HTML with the toolbar/script embeds, link rewriting, and
// display-only CSS (original-page style re-embed, reader.css, domain-specific patches) all
// omitted, so agents can isolate the article body without paying the token cost of markup
// they have no use for. On failure, the response is always a JSON body of the form
// {"error": "...", "reason": "..."} (see the reason* constants) with an appropriate 4xx/5xx
// status code, so callers never mistake a failure for a successful fetch.
func (h *Handler) HandleProxy(w http.ResponseWriter, r *http.Request) {
	programMode := isProgramMode(r)

	targetURL, err := parseTargetURL(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, reasonInvalidURL, err.Error(), 0)
		return
	}

	select {
	case concurrentSemaphore <- struct{}{}:
		defer func() { <-concurrentSemaphore }()
	case <-r.Context().Done():
		writeJSONError(w, http.StatusServiceUnavailable, reasonOverloaded, "server busy", 0)
		return
	}

	startTime := time.Now()

	ctx, ctxCancel := chromedp.NewContext(h.allocatorContext)
	defer ctxCancel()
	ctx, timeCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeCancel()

	userAgent := renderUserAgent(r, programMode)

	rawHTML, cssTexts, totalNetworkBytes, upstreamStatus, err := renderPage(ctx, targetURL, userAgent)
	if err != nil {
		reason, statusCode, message := classifyRenderError(err)
		slog.Error("chrome render failed", slog.String("url", targetURL), slog.String("reason", reason), slog.Any("error", err))
		writeJSONError(w, statusCode, reason, message, 0)
		return
	}

	if upstreamStatus >= 400 {
		slog.Warn("target site returned an error status", slog.String("url", targetURL), slog.Int("status", upstreamStatus))
		writeJSONError(w, http.StatusBadGateway, reasonRenderBlocked, fmt.Sprintf("target site responded with status %d", upstreamStatus), upstreamStatus)
		return
	}

	processedHTML, err := h.processHTML(rawHTML, targetURL, cssTexts, programMode)
	if err != nil {
		slog.Error("html processing failed", slog.String("url", targetURL), slog.Any("error", err))
		writeJSONError(w, http.StatusInternalServerError, reasonProcessingFailed, "html processing error", 0)
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

// renderUserAgent returns the User-Agent that headless Chrome presents to the target site.
//
// Browser callers get their own User-Agent so that sites serve the layout suited to the
// device they browse from (e.g. mobile pages for a phone). Program-mode callers always get
// defaultUserAgent instead: they are HTTP clients such as curl or Node's fetch whose own
// User-Agent makes sites like DuckDuckGo answer with a bot challenge instead of content.
func renderUserAgent(r *http.Request, programMode bool) string {
	if programMode {
		return defaultUserAgent
	}
	if ua := r.UserAgent(); ua != "" {
		return ua
	}
	return defaultUserAgent
}

// isProgramMode reports whether the caller requested the lightweight, AI agent-oriented
// response (no toolbar, no link rewriting, no display-only CSS) via the "X-Program-Mode"
// request header.
func isProgramMode(r *http.Request) bool {
	v := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Program-Mode")))
	return v == "true" || v == "1"
}

// classifyRenderError maps a renderPage error to a machine-readable reason, HTTP status
// code, and human-readable message.
func classifyRenderError(err error) (reason string, statusCode int, message string) {
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline exceeded") {
		return reasonRenderTimeout, http.StatusGatewayTimeout, "render timed out"
	}
	return reasonRenderFailed, http.StatusBadGateway, fmt.Sprintf("render error: %v", err)
}

// writeJSONError writes a structured JSON error response so callers can reliably detect
// and classify a failed request instead of mistaking it for a successful fetch.
func writeJSONError(w http.ResponseWriter, statusCode int, reason, message string, upstreamStatus int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(errorResponse{
		Error:  message,
		Reason: reason,
		Status: upstreamStatus,
	}); err != nil {
		slog.Error("failed to encode error response", slog.Any("error", err))
	}
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
