# bare-web-proxy Agent Rules

このファイルは、`bare-web-proxy` リポジトリで作業する AI Agent 向けの動作ルールと前提条件を定義する。全体像は `README.md` を参照する。

## 1. ディレクトリ構成

| パス | 役割 |
| :-- | :-- |
| `cmd/proxy/main.go` | エントリポイント。ルーティングのみ |
| `internal/proxy/resolver.go` | `?q=` を実ターゲットURLへ解決 |
| `internal/proxy/render.go` | chromedp でページをレンダリングしHTML/CSSを取得。アセット遮断も担う |
| `internal/proxy/process.go` | goquery による HTML 加工（タグ除去 → CSS再注入 → リンク書き換え → ツールバー注入 → `modifiers` 適用） |
| `internal/proxy/modifiers/` | ドメイン固有の表示崩れ対策パッチ |
| `internal/proxy/static/` | フロントエンドのビルド成果物。**手で編集しない**（`frontend/` からビルドして生成） |
| `frontend/` | ツールバー・リーダーモードCSSのソース（`npm run build`） |
| `k8s/` | Deployment/Service/Ingress 定義（`base` / `overlays/prod`）。`overlays/local` は `benchmark/` 用でAgentの検証には使わない |

## 2. 表示崩れ調査の当たりの付け方

- **JS依存レイアウト**: `stripTags` は script/img/video 等を除去するため、JS依存のSPAはレンダリング後の変化が反映されないことがある
- **アセット遮断**: `render.go` の `blockedURLPatterns` が画像・フォント等を遮断しており、それに依存したレイアウトは崩れうる
- **CSS再適用漏れ**: `injectCSS` はCSSOMのスナップショットのみを再注入するため、`:hover` 等の擬似クラスやCSS-in-JSは拾いきれない
- 上記に当てはまらないドメイン固有の崩れは `modifiers/`（例: `zenn.go`）にパッチを追加する
- リンク遷移の異常は `process.go` の `rewriteLinks` と `resolver.go` の URL 判定を疑う
- ツールバー等が古い場合は `internal/proxy/static/` が `frontend/` の最新ビルドと同期しているか確認する

## 3. 修正後の検証方法

```bash
go build ./...
go vet ./...
```

`frontend/` を変更した場合は `npm ci && npm run typecheck && npm run build` も行う。ビルド成果物を `internal/proxy/static/` へ手動コピーする運用は取らない。Dockerビルドの成否はCIで確認するためローカルでは行わない。

`kind` によるローカル起動確認はAgentでは行わない。表示崩れの実地検証が必要な場合は、変更内容と確認観点をPR上で説明し、人間によるレビューを依頼する。

## 4. 対応済みの制約

`nuage-workspace/AGENTS.md` に定義された横断ルールに準じる。

- **言い切り調の使用**: です・ます調を避け、である・する調で記述する
- **コミット規約**: `<種別>: <変更内容>`（`add:`, `update:`, `fix:`, `docs:` 等）
- **GitOps 原則**: `k8s/` 配下の変更は `master` へ push すれば Argo CD が自動同期する
- **SOPS / Terraform, Terragrunt 操作の禁止**: 特別な指示がない限り実行しない
