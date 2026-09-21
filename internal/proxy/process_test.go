package proxy

import (
	"strings"
	"testing"
)

const sampleHTML = `<html><head></head><body>
<a href="https://example.com/article">absolute</a>
<a href="/relative/path">relative</a>
</body></html>`

func TestProcessHTMLDefaultModeRewritesLinksAndInjectsToolbar(t *testing.T) {
	h := &Handler{}
	out, err := h.processHTML(sampleHTML, "https://example.com/", nil, false)
	if err != nil {
		t.Fatalf("processHTML() error = %v", err)
	}

	if !strings.Contains(out, `id="proxy-toolbar-container"`) {
		t.Errorf("expected toolbar container to be injected, got: %s", out)
	}
	if !strings.Contains(out, `href="/proxy?url=https%3A%2F%2Fexample.com%2Farticle"`) {
		t.Errorf("expected absolute link to be rewritten to a proxy relay link, got: %s", out)
	}
	if !strings.Contains(out, `href="/proxy?url=https%3A%2F%2Fexample.com%2Frelative%2Fpath"`) {
		t.Errorf("expected relative link to be resolved and rewritten to a proxy relay link, got: %s", out)
	}
}

func TestProcessHTMLProgramModeSkipsRewriteLinksAndToolbar(t *testing.T) {
	h := &Handler{}
	out, err := h.processHTML(sampleHTML, "https://example.com/", nil, true)
	if err != nil {
		t.Fatalf("processHTML() error = %v", err)
	}

	if strings.Contains(out, `id="proxy-toolbar-container"`) {
		t.Errorf("expected toolbar container to be omitted in program mode, got: %s", out)
	}
	if strings.Contains(out, "/proxy?url=") {
		t.Errorf("expected links to be left unrewritten in program mode, got: %s", out)
	}
	if !strings.Contains(out, `href="https://example.com/article"`) {
		t.Errorf("expected original absolute href to be preserved, got: %s", out)
	}
	if !strings.Contains(out, `href="/relative/path"`) {
		t.Errorf("expected original relative href to be preserved as-is, got: %s", out)
	}
}

func TestProcessHTMLDefaultModeInjectsCSS(t *testing.T) {
	h := &Handler{}
	out, err := h.processHTML(sampleHTML, "https://example.com/", []string{"body { color: red; }"}, false)
	if err != nil {
		t.Fatalf("processHTML() error = %v", err)
	}

	if !strings.Contains(out, `<style data-proxy-style="original">`) {
		t.Errorf("expected original CSS to be re-embedded in default mode, got: %s", out)
	}
	if !strings.Contains(out, `<link rel="stylesheet" id="proxy-reader-style"`) {
		t.Errorf("expected reader.css stylesheet link in default mode, got: %s", out)
	}
}

func TestProcessHTMLProgramModeSkipsCSS(t *testing.T) {
	h := &Handler{}
	out, err := h.processHTML(sampleHTML, "https://example.com/", []string{"body { color: red; }"}, true)
	if err != nil {
		t.Fatalf("processHTML() error = %v", err)
	}

	if strings.Contains(out, "<style") {
		t.Errorf("expected no <style> tags in program mode, got: %s", out)
	}
	if strings.Contains(out, `rel="stylesheet"`) {
		t.Errorf("expected no stylesheet links in program mode, got: %s", out)
	}
}

func TestProcessHTMLProgramModeSkipsDomainModifiers(t *testing.T) {
	h := &Handler{}
	out, err := h.processHTML(sampleHTML, "https://zenn.dev/", nil, true)
	if err != nil {
		t.Fatalf("processHTML() error = %v", err)
	}

	if strings.Contains(out, `id="proxy-domain-patch-zenn"`) {
		t.Errorf("expected domain-specific CSS patch to be skipped in program mode, got: %s", out)
	}
}

func TestProcessHTMLDefaultModeAppliesDomainModifiers(t *testing.T) {
	h := &Handler{}
	out, err := h.processHTML(sampleHTML, "https://zenn.dev/", nil, false)
	if err != nil {
		t.Fatalf("processHTML() error = %v", err)
	}

	if !strings.Contains(out, `id="proxy-domain-patch-zenn"`) {
		t.Errorf("expected domain-specific CSS patch to be applied in default mode, got: %s", out)
	}
}
