package proxy

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"net/http"
	"path"
)

//go:embed static
var staticFS embed.FS

type assetEntry struct {
	content     []byte
	contentType string
	etag        string
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

func (h *Handler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	entry := h.assetMap["index.html"]
	w.Header().Set("Content-Type", entry.contentType)
	w.Write(entry.content)
}

func (h *Handler) HandleAssets(w http.ResponseWriter, r *http.Request) {
	filename := path.Base(r.URL.Path)
	entry, ok := h.assetMap[filename]
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("ETag", `"`+entry.etag+`"`)
	if r.Header.Get("If-None-Match") == `"`+entry.etag+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", entry.contentType)
	w.WriteHeader(http.StatusOK)
	w.Write(entry.content)
}
