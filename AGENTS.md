# プロジェクト規約

このリポジトリは、Go、Echo、templ、is-land、Preact/htm、Tailwind CSS、daisyUI、SQLite、CertMagic、lipgloss、tint、huhで構成するワンバイナリのフルスタックWebアプリケーション・テンプレートである。

- ワンバイナリ: 別ランタイム不要
- デプロイ: systemd、distrolessコンテナ、Nanosユニカーネルなど
- 開発者体験: `just run`でホットリロード・TUI対応
- Islands Architecture: templ + `<is-land>` + Preact/htm (ビルド時・実行時 Node.js不要)

小さなpackage境界を保つ。

## AGENTS.md / README.md　の参照と更新

- ユーザの認識・意見と`AGENTS.md`や`README.md`の記述との間で矛盾やズレが生じている時、AIの認識の根拠となった`AGENTS.md`/`README.md`の記述を明示し、根拠を提示する。
- 間違った記述だと指摘された場合は、その記述を削除/修正する。

## 不変条件

- `cmd/server/main.go`にはCLI解析と終了処理だけを置く。
- 依存関係は`internal/app`だけで組み立てる。
- routeは`internal/web/router.go`から明示的に辿れるようにする。
- `init()`やグローバルregistryでrouteや依存関係を登録しない。
- Echo、templ、islandのマウントHTMLへの依存は`internal/web`に閉じ込める。
- 業務操作は`internal/feature/<name>`に置き、Web handlerからSQLを直接実行しない。
- DB接続(driver/DSN)、Bunの構築、`bun/migrate`の実行順序は`internal/db`が所有する。
- 機能packageのデータアクセスにはBunを使い、schema変更は`internal/db/migrations/<driver>`の連番SQL migrationで管理する。
- 操作の入口は`account.Accounts`のように機能を表す名前にし、汎用的な`Service`や`Context`を避ける。
- HTTP、HTTPS、CertMagicの起動処理は`internal/httpserver`が所有する。
- 起動バナー、tint付き端末ログは`internal/terminal`が所有する。
- 開発用ランチャー／TUI(Bubble Tea)は`cmd/dev/internal/tui`が所有する。
- プロジェクト設定(`.unigo.toml`)は`internal/config`が所有する。
- 埋め込み静的資産は`static`が所有する(CSS、vendored ESM)。islandのソースは`internal/web/islands`が所有し、`/static/islands`として配信する。
- カスタムバナーロゴのASCII生成は`cmd/logo` / `internal/logogen`が所有する(`bin/server`にはascii-image-converterをリンクしない)。
- `common`、`utils`、`service`、`repository`、`cli`を慣習だけで追加しない。
- interfaceは必要とするconsumer側に最小の形で定義する。
- migrationは`*.tx.up.sql`と`*.tx.down.sql`を対にし、適用後に変更せず新しい連番ファイルを追加する。
- `*_templ.go`は生成物なので直接編集しない。
- importはファイル先頭に置き、関数内importを使わない。

## Web UI(templ / is-land / Preact / daisyUI)

- ページのドキュメントシェルはtempl、リアクティブなislandは`internal/web/islands`のESM(Preact + htm)に閉じる。
- islandの依存(is-land、Preact、htm、preact-custom-element)は`static/vendor`へvendoringし、`go:embed`する。実行時CDNやNodeビルドチェーンに依存しない。再取得は`just vendor-islands`。
- ページ内のインライン`<script>`で業務ロジックや`localStorage`操作を増やさない。必要ならislandのESMへ置く。
- テーマ切替はdaisyUIの`theme-controller`(CSSのみ)を使う。
- daisyUIにコンポーネントがある操作は、その公式パターンを優先する。

## 機能追加

1. `internal/feature/<name>`へ業務APIを追加する。
2. `internal/db/migrations/<driver>`へschema変更を追加する。
3. `internal/web/<area>`へhandlerとtemplを追加する(islandが必要なら`internal/web/islands`も)。
4. `internal/app`で依存関係を構築する。
5. `internal/web/router.go`へrouteを登録する。

## 変更後の確認

```sh
go tool templ generate -path internal/web
just css
go test ./...
go vet ./...
just build
```

## 文書の表記

- island / Islands / is-landは訳さず英語のまま書く。「島」と日本語化しない。
- 日本語の文中に英単語やコード片を入れるとき、その前後に半角スペースを入れない(例: `templの生成`、`Preactを使用`)。
- 括弧は半角`()`を使い、全角`（）`は使わない。
- 半角`:`を使い、全角`：`を使わない