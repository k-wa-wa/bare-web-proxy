package modifiers

import (
	"github.com/PuerkitoBio/goquery"
)

// modifyZenn は Zenn におけるスマホ表示時のスクロール不能問題を解決します。
func modifyZenn(doc *goquery.Document) {
	// JSなしによる初期 overflow: hidden を強制上書き
	css := `
html, body, #__next, [role="main"] {
	overflow: visible !important;
	overflow-y: visible !important;
	height: auto !important;
	position: static !important;
	touch-action: auto !important;
}
`
	doc.Find("head").AppendHtml("<style id=\"proxy-domain-patch-zenn\">\n" + css + "\n</style>")
}
