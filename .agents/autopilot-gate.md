PR の Preview 環境に対して以下を実際に実行して確認する。

## 検証対象

PR ごとの Preview 環境 `https://bwproxy-pr-<PR番号>.wpcapp.net` に対して HTTP リクエストを送る。

```bash
BASE="https://bwproxy-pr-<PR番号>.wpcapp.net"
```

Preview は PR への push から数分後にデプロイされる。接続できない場合は 60 秒間隔で最大 5 回リトライする。

```bash
for i in 1 2 3 4 5; do
  code=$(curl -sS -o /dev/null -w "%{http_code}" --max-time 15 "${BASE}/" || echo "000")
  echo "attempt $i: HTTP $code"
  [ "$code" != "000" ] && break
  sleep 60
done
```

5 回とも `000`（名前解決失敗・接続拒否・タイムアウト）の場合、コードの良否は判定できない。
以降の確認は行わず、どのコマンドがどう失敗したかを事実として報告する。

## 確認項目

### 1. 実サイトのレンダリング（変更内容によらず毎回）

`X-Program-Mode` ヘッダなし（既定のブラウザ向け）と、あり（`true`）の両方で、
同じサイトが描画されることを確認する。ヘッダなしは既存の利用形態であり、変更が
無いはずの場合でも回帰を見るため必ず実施する。

```bash
check() { # $1=mode(default|prog) $2=url $3=ページ内に必ず含まれる文字列
  local hdr=(); [ "$1" = prog ] && hdr=(-H "X-Program-Mode: true")
  for try in 1 2; do  # 描画待ちの取りこぼしがまれにあるため、失敗時のみ 1 回再試行する
    code=$(curl -sS --max-time 40 "${hdr[@]}" "${BASE}/proxy?url=$2" -o /tmp/gate.html -w "%{http_code}")
    [ "$code" = 200 ] && grep -q -- "$3" /tmp/gate.html && { echo "OK: $1 $2"; return; }
  done
  echo "NG: $1 $2 (HTTP $code, size $(wc -c < /tmp/gate.html))"; head -c 300 /tmp/gate.html; echo
}

for mode in default prog; do
  check $mode https://example.com "Example Domain"                                # 最小のページ
  check $mode https://news.ycombinator.com/ "Hacker News"                         # 素の HTML・リンク書き換え
  check $mode https://ja.wikipedia.org/wiki/Go "プログラミング言語"                # 大きいページ・CSS 再適用
  check $mode https://zenn.dev/ "Zenn"                                            # ドメイン固有の修飾処理の対象
  check $mode https://todomvc.com/examples/react/dist/ "todos"                    # JS 実行後にしか現れない SPA
done
```

- すべて OK であること。NG は不合格とし、出力された HTTP ステータス・サイズ・応答の冒頭を記録する。
- 上記サイトは変更しやすい実サイトなので、文字列の不一致だけでなく、HTTP ステータスとサイズも見て、
  サイト側の変更か bwproxy の不具合かを切り分けて報告する。

### 2. 既定モードの基本形

ヘッダなしの応答は、ツールバー付きで、リンクが bwproxy 経由に書き換えられていること。

```bash
curl -sS --max-time 40 "${BASE}/proxy?url=https://example.com" -o /tmp/gate-default.html
grep -q 'id="proxy-toolbar-container"' /tmp/gate-default.html && echo "OK: toolbar" || echo "NG: toolbar が無い"
curl -sS --max-time 40 "${BASE}/proxy?url=https://news.ycombinator.com/" | grep -q 'href="/proxy?url=' && echo "OK: links" || echo "NG: links が書き換えられていない"
```

### 3. README に記載された外部仕様と、この PR の変更点

- README の `/proxy` の外部仕様（リクエストヘッダ・エラー応答の形式・ステータスコード）に記載された挙動を、
  実際に curl で確認する。README と食い違う場合は不合格とする。
- この PR が変更・追加した挙動は、差分を読み、同じ要領で curl して確認する。
  正常系だけでなく、その変更が影響しうる異常系（不正な `url`、存在しない URL など）も 1 つ以上叩く。
