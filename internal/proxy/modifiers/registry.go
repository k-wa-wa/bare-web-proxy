package modifiers

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// DocumentModifier は特定のドメインに対して HTML ドキュメントを変更する関数型です。
type DocumentModifier func(doc *goquery.Document)

// domainModifiers にドメイン名とそれに対応するモディファイア関数を登録します。
var domainModifiers = map[string]DocumentModifier{
	"zenn.dev": modifyZenn,
}

// ModifyDocument は指定されたURLのホスト名に基づき、登録されたモディファイアを適用します。
func ModifyDocument(doc *goquery.Document, targetURL string) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return
	}
	host := u.Hostname()

	for domain, modifier := range domainModifiers {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			modifier(doc)
		}
	}
}
