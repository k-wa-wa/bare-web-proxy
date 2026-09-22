package proxy

import (
	"strings"
	"testing"
)

const sampleHTML = `<html><head></head><body>
<a href="https://example.com/article">absolute</a>
<a href="/relative/path">relative</a>
</body></html>`

const decoratedHTML = `<html><head></head><body>
<!-- top comment -->
<div id="main" class="article" style="color: red;" data-testid="root" data-foo="bar">
	<p class="text">hello<!-- inline comment --></p>
</div>
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

const structuralHTML = `<html><head></head><body>
<header id="site-header">header content</header>
<nav id="site-nav">nav content</nav>
<aside id="site-aside">aside content</aside>
<main id="site-main">main content</main>
<footer id="site-footer">footer content</footer>
</body></html>`

func TestProcessHTMLProgramModeStripsStructuralTags(t *testing.T) {
	h := &Handler{}
	out, err := h.processHTML(structuralHTML, "https://example.com/", nil, true)
	if err != nil {
		t.Fatalf("processHTML() error = %v", err)
	}

	for _, id := range []string{"site-header", "site-nav", "site-aside", "site-footer"} {
		if strings.Contains(out, id) {
			t.Errorf("expected element with id=%s to be stripped in program mode, got: %s", id, out)
		}
	}
	if !strings.Contains(out, "site-main") {
		t.Errorf("expected main content to be preserved in program mode, got: %s", out)
	}
}

func TestProcessHTMLDefaultModeKeepsStructuralTags(t *testing.T) {
	h := &Handler{}
	out, err := h.processHTML(structuralHTML, "https://example.com/", nil, false)
	if err != nil {
		t.Fatalf("processHTML() error = %v", err)
	}

	for _, id := range []string{"site-header", "site-nav", "site-aside", "site-footer", "site-main"} {
		if !strings.Contains(out, id) {
			t.Errorf("expected element with id=%s to be preserved in default mode, got: %s", id, out)
		}
	}
}

func TestProcessHTMLProgramModeStripsPresentationalAttrsAndComments(t *testing.T) {
	h := &Handler{}
	out, err := h.processHTML(decoratedHTML, "https://example.com/", nil, true)
	if err != nil {
		t.Fatalf("processHTML() error = %v", err)
	}

	for _, want := range []string{"class=", "style=", "data-testid", "data-foo", "<!--"} {
		if strings.Contains(out, want) {
			t.Errorf("expected %q to be stripped in program mode, got: %s", want, out)
		}
	}
	if !strings.Contains(out, `id="main"`) {
		t.Errorf("expected non-presentational attributes like id to be preserved, got: %s", out)
	}
}

func TestProcessHTMLDefaultModeKeepsPresentationalAttrsAndComments(t *testing.T) {
	h := &Handler{}
	out, err := h.processHTML(decoratedHTML, "https://example.com/", nil, false)
	if err != nil {
		t.Fatalf("processHTML() error = %v", err)
	}

	for _, want := range []string{`class="article"`, `style="color: red;"`, `data-testid="root"`, `data-foo="bar"`, "<!-- top comment -->", "<!-- inline comment -->"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q to be preserved in default mode, got: %s", want, out)
		}
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
