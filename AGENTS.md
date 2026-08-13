# プロジェクト規約

このリポジトリは**unagi**(個人用のミニマルな技術ブログ)である。[unigo-template](https://github.com/SouichiroTsujimoto/unigo-template)から作成し、同じワンバイナリ構成(Go、Echo、templ、is-land、Preact/htm、Tailwind CSS、daisyUI、lipgloss、tint、huh)を引き継ぐ。DB / Auth / StorageはSupabase、本番はVercel Go Runtimeとする。

- ワンバイナリ: 別ランタイム不要(Vercelが`go.mod`のtoolchainでbuild)
- デプロイ: Vercelが独自ドメインとTLSを終端し、Go Framework PresetのHTTPサーバ(東京`hnd1`)を動かす。mainへのpushでVercel Git integrationがデプロイし、Supabase GitHub integration(Deploy to production)が`supabase/migrations/`を適用する。手順は`deploy/README.md`
- 開発者体験: `supabase start` + `just run`でホットリロード・TUI対応。`cmd/dev`は`.env`を読む(`bin/server`は読まない)
- Islands Architecture: templ + `<is-land>` + Preact/htm (ビルド時・実行時 Node.js不要)。supabase-jsはislandに入れない
- 記事・画像の正本はGitHubリポジトリ`SouichiroTsujimoto/unagi-content`。Postgresは公開サイトのread model
- 管理画面(`/admin`)はSupabase AuthのX OAuth + `UNIGO_ADMIN_USER_IDS`のallowlist。公開切替とコメント・ステッカー管理だけを行う。秘密は環境変数(`.unigo.toml`に平文で置かない)
- 記事同期は`UNIGO_CONTENT_SYNC_SECRET`のHMACと固定repository`SouichiroTsujimoto/unagi-content`。Gitから消えた記事はDBから物理削除され、コメント・ステッカーもcascadeで消える。`images/`から消えたファイルに対応するStorageオブジェクトも同期時に削除する

小さなpackage境界を保つ。

## AGENTS.md / README.md　の参照と更新

- ユーザの認識・意見と`AGENTS.md`や`README.md`の記述との間で矛盾やズレが生じている時、AIの認識の根拠となった`AGENTS.md`/`README.md`の記述を明示し、根拠を提示する。
- 間違った記述だと指摘された場合は、その記述を削除/修正する。

## バージョン管理(Jujutsu)

- このリポジトリの作業VCSはGitとcolocatedなJujutsu(`jj`)である。
- 履歴を書き換える操作は`git`ではなく`jj`を使う(`status` / `diff` / `log` / `commit` / `describe` / `rebase` / `squash` / `bookmark` / `git push` / `git fetch`)。
- `git`は読み取りや`gh`連携の補助に留め、mutatingな`git commit` / `git rebase` / `git push`は避ける。
- bookmarkはGit branchと同期する。push前に必要なbookmarkを`@-`へ動かす(例: `jj bookmark move main --to @-`)。名前を自動生成してpushする場合は`jj git push -c @-`。
- 意味のある区切りがついたら、依頼を待たず`jj describe`と`jj new`を行う。pushとbookmark移動はユーザが明示したときだけ。秘密情報を含めず、`main`へのforce相当や破壊的な操作はユーザが明示しない限り行わない。実装は`jj`で行う。
- `.jj/`とユーザの`~/.config/jj`は編集・コミットしない。

## 不変条件

- `cmd/server/main.go`にはCLI解析と終了処理だけを置く。
- 依存関係は`internal/app`だけで組み立てる。
- routeは`internal/web/router.go`から明示的に辿れるようにする。
- `init()`やグローバルregistryでrouteや依存関係を登録しない。
- Echo、templ、islandのマウントHTMLへの依存は`internal/web`に閉じ込める。
- 業務操作は`internal/feature/<name>`に置き、Web handlerからSQLを直接実行しない。
- 機能packageのデータアクセスにはBunを使う。
- DB接続(driver/DSN)とBunの構築は`internal/db`が所有する。schema変更は`supabase/migrations/`のtimestamp付きSQLで管理し、適用はSupabase CLIとGitHub integrationに任せる。アプリ起動時には流さない。
- 操作の入口は`article.Articles`や`auth.Auth`のように機能を表す名前にし、汎用的な`Service`や`Context`を避ける。
- HTTPサーバの起動処理(Vercel Go Runtime / ローカル向けHTTP + SIGTERM)は`internal/httpserver`が所有する。
- 起動バナー、tint付き端末ログは`internal/terminal`が所有する。
- 開発用ランチャー／TUI(Bubble Tea)は`cmd/dev/internal/tui`が所有する。
- プロジェクト設定(`.unigo.toml`)は`internal/config`が所有する。site descriptionは`internal/app`の固定値とし、secretは環境変数 / `.env`。
- 埋め込み静的資産は`static`が所有する(CSS、vendored ESM)。islandのソースは`internal/web/islands`が所有し、`/static/islands`として配信する。
- カスタムバナーロゴのASCII生成は`cmd/logo` / `internal/logogen`が所有する(`bin/server`にはascii-image-converterをリンクしない)。
- `common`、`utils`、`service`、`repository`、`cli`を慣習だけで追加しない。
- interfaceは必要とするconsumer側に最小の形で定義する。
- schemaのmigrationは`supabase/migrations/`へtimestamp付きSQLを追加する。適用後のファイルは変更せず、新しいファイルを足す。
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
2. `supabase/migrations/`へschema変更を追加する。
3. `internal/web/<area>`へhandlerとtemplを追加する(islandが必要なら`internal/web/islands`も)。
4. `internal/app`で依存関係を構築する。
5. `internal/web/router.go`へrouteを登録する。

## 変更後の確認

`just run`が動いていないとき:

```sh
go tool templ generate -path internal/web
just css
go test ./...
go vet ./...
just build
```

`just run`実行中(air / css-watchが監視しているとき)は、ソースを編集すればホットリロードされる。次のリビルド系コマンドは走らせない(出力がぶつかり干渉する)。

- `go tool templ generate` / `just generate`
- `just css` / `just css-watch`
- `just build` / `just check`

`go test`や`go vet`は実行してよい。terminalsに`just run`やair / css-watchのactive commandがあるかで判定する。

## 文書の表記

- island / Islands / is-landは訳さず英語のまま書く。「島」と日本語化しない。
- 日本語の文中に英単語やコード片を入れるとき、その前後に半角スペースを入れない(例: `templの生成`、`Preactを使用`)。
- 括弧は半角`()`を使い、全角`（）`は使わない。
- 半角`:`を使い、全角`：`を使わない
