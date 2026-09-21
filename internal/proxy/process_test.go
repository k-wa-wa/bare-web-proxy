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
