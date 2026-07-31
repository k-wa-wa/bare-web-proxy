# bare-web-proxy Agent Rules

このファイルは、`bare-web-proxy` リポジトリで作業する AI Agent 向けの動作ルールと前提条件を定義する。全体像は `README.md` を参照し、本ファイルは調査・修正・検証の実務に必要な情報に絞る。

## 1. ディレクトリ構成の要点

| パス | 役割 |
| :-- | :-- |
| `cmd/proxy/main.go` | エントリポイント。`/`, `/proxy`, `/proxy/assets/` の3ルートを登録するのみで、実処理は `internal/proxy` 側にある |
| `internal/proxy/resolver.go` | `?q=` パラメータ（URL / ドメイン / 検索語のいずれか）を解決して実際のターゲットURLへ変換する |
| `internal/proxy/render.go` | chromedp（Headless Chrome）でターゲットページをレンダリングし、生HTMLと適用済みCSSを取得する。画像・動画・フォント・広告トラッカーの通信は `blockedURLPatterns` で遮断している |
| `internal/proxy/process.go` | goquery による HTML 加工本体。`stripTags`（script/style/img等の除去）→ `injectCSS`（レンダリング結果のCSSを再注入）→ `rewriteLinks`（`<a>` を `/proxy?url=...` 形式に書き換え）→ `injectToolbar`（ツールバー注入）→ `modifiers.ModifyDocument`（ドメイン別パッチ）の順で処理する |
| `internal/proxy/modifiers/` | ドメイン固有の表示崩れ対策。`registry.go` のホスト名マップに `domain: 関数` を追加する形で拡張する（実例: `zenn.go`） |
| `internal/proxy/assets.go` | `internal/proxy/static/` を `go:embed` し、`/`（index.html）と `/proxy/assets/*`（toolbar.js, reader.css）として配信する |
| `internal/proxy/static/` | フロントエンドのビルド成果物。**手で編集しない**。`frontend/` からビルドして上書きする資材（後述） |
| `frontend/` | ツールバー（`toolbar.ts`, `toolbar.css`）とリーダーモードCSS（`reader.css`）のソース。`npm run build`（`build.mjs`、esbuildベース）で `frontend/dist/` に出力し、Docker ビルド時に `internal/proxy/static/` へコピーされる |
| `k8s/base/` | Deployment（`proxy-app` + `headless-chrome` のサイドカー構成）、Service の定義 |
| `k8s/overlays/prod/` | 本番向け Ingress（`bwproxy.cluster.wpc`、TLS） |
| `k8s/overlays/local/` | `kind` によるローカル検証用オーバーレイ、モックサーバー定義 |
| `benchmark/` | 通信量削減効果の負荷試験用クライアント・モックサーバー |
| `scripts/deploy-local.sh` / `scripts/patch-dns.sh` | `kind` クラスタへのローカルデプロイとDNSパッチ（後述） |

## 2. 表示崩れの原因になりやすい箇所の当たりの付け方

「特定サイトの表示が崩れる」という報告を受けたら、まず以下の順で切り分ける。

1. **JS依存レイアウトの欠落**:
   `process.go` の `stripTags` は `script, noscript, iframe, img, svg, video, style, link[rel='stylesheet']` を丸ごと除去する。JSが動的にDOM構造やクラスを付与するサイト（SPA、遅延ハイドレーション系）は、レンダリング後のスナップショットにその変化が反映されないことがある。まずはブラウザで対象URLを直接開き、JS無効化状態（DevToolsでJS disable）でも同じ崩れが起きるか確認する。
2. **ブロック対象アセットへの依存**:
   `render.go` の `blockedURLPatterns` は画像・動画・音声・フォント・広告/アナリティクス系ドメインを一律ブロックしている。レイアウトが `img`/`font` のサイズに依存している（`aspect-ratio` 未指定の画像など）場合、要素の除去とサイズ喪失でガタつきが起きる。対象サイトの通信を見て、どのリソースがブロックされ、それが崩れの原因になっているか切り分ける。
3. **CSSの再適用漏れ・競合**:
   `injectCSS` は Headless Chrome が実際に適用した CSSOM のテキストをそのまま `<style data-proxy-style="original">` として head に差し込む。`@import`、CSS-in-JS が生成した動的クラス、`:hover` 等の擬似クラス依存スタイルはこの方式では拾いきれない。また `reader.css` や `toolbar.css` との詳細度衝突（`!important` の有無）でも崩れが起きうる。
4. **ドメイン固有の問題**:
   上記のいずれにも当てはまらず、特定ドメイン特有の挙動（例: `overflow: hidden` がJS実行前提で解除される等）が原因の場合は、`internal/proxy/modifiers/` にドメイン別の対症パッチを追加する。既存の `zenn.go` を実装例として参照し、`registry.go` の `domainModifiers` にホスト名を登録する。
5. **リンク遷移の崩れ**:
   ページ内リンクや相対パスの挙動がおかしい場合は `process.go` の `rewriteLinks` / `resolveHref`、および `resolver.go` の URL 判定ロジック（ドメイン判定 vs 検索語判定）を疑う。
6. **静的アセット配信の不整合**:
   ツールバーやリーダーモードCSSそのものが当たっていない・古い場合は、`internal/proxy/static/` が `frontend/` の最新ビルド成果物と同期しているかを確認する（下記「検証方法」参照）。ルーティングは `cmd/proxy/main.go` の3ルート（`/`, `/proxy`, `/proxy/assets/`）のみなので、パスの不一致による404もここで切り分ける。

## 3. 修正後の検証方法

### ローカルビルド確認（最低限これは必須）

```bash
go build ./...
go vet ./...
```

`frontend/` を変更した場合は、あわせてビルドと型チェックを行う。

```bash
cd frontend
npm ci
npm run typecheck
npm run build   # dist/ に成果物が出る
```

`frontend/` の変更を実際に配信へ反映するには、Docker ビルド（後述）で `internal/proxy/static/` に取り込ませる。`frontend/dist/` の成果物を `internal/proxy/static/` に手動コピーする運用は取らない。

### Docker ビルド確認

```bash
go mod tidy
go mod vendor
docker build -t bare-web-proxy:local .
```

`Dockerfile` はマルチステージ（frontend-builder → Goビルド → scratch実行イメージ）になっており、`internal/proxy/static/` は Go ビルドステージで `frontend-builder` の成果物によって上書きされる。

### kind によるローカル起動確認（表示崩れの実地検証にはこちらを使う）

`scripts/deploy-local.sh` が `go mod vendor` → Docker ビルド → `kind` クラスタ作成/更新 → `k8s/overlays/local` 適用 → `localhost:3000` へのポートフォワードまで一括で行う。

```bash
bash scripts/deploy-local.sh
```

対象サイトへの名前解決がクラスタ内から通らない場合は `scripts/patch-dns.sh` で CoreDNS のフォワード先をホストゲートウェイに向ける。

起動後はブラウザで実際に対象URLを開いて確認する。

```
http://localhost:3000/proxy?url=<検証したい対象URLをURLエンコード>
```

ツールバーの「オリジナルCSS適用トグル」を切り替えて、リーダーモード/オリジナルCSS両方の見え方を目視確認する。ポートフォワードのログは `port-forward.log` に出力される。

## 4. 対応済みの制約

`nuage-workspace/AGENTS.md` に定義された横断ルールに準じる。特に以下は本リポジトリでも厳守する。

- **言い切り調の使用**: コメントやドキュメントは です・ます調を避け、である・する調で記述する
- **コミット規約**: 本リポジトリの `git log` は `<種別>: <変更内容>`（例: `add:`, `update:`, `fix:`, `clean:`, `refactor:`, `docs:`）という接頭辞を用いる慣習で運用されている。新規コミットもこれに従う
- **GitOps 原則**: `k8s/` 配下の変更は `master` へ push すれば Argo CD が自動同期する。稼働中クラスタへの `kubectl edit` 等の直接編集は恒久対応にしない
- **SOPS / Terraform, Terragrunt 操作の禁止**: 特別な指示がない限り実行しない
