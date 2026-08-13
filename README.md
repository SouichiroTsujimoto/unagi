# unagi

個人用のミニマルな技術ブログ。Goワンバイナリ(Echo / templ / is-land / Preact) + Supabase(Postgres / Auth / Storage) + Cloud Run(distroless)。

[unigo-template](https://github.com/SouichiroTsujimoto/unigo-template)から作成しています。

設計の不変条件は[AGENTS.md](AGENTS.md)。Cloud Run手順は[deploy/cloudrun/README.md](deploy/cloudrun/README.md)。

## 開発

```sh
cp .env.example .env
supabase gen signing-key --algorithm ES256 > supabase/signing_keys.json
supabase start
# `supabase start` の Publishable / Secret を .env に反映
# Studioで管理者ユーザを作り、UUIDを UNIGO_ADMIN_USER_IDS へ
just run
```

`http://localhost:8080`。`just run`は`.env`を読む(`bin/server`は読まない)。秘密は`.unigo.toml`に置かない。

```sh
just run                 # logo + TUI(既定)
just run tui=false
just build && bin/server
```

## バージョン管理

このリポジトリはGitと[Jujutsu](https://docs.jj-vcs.dev/)(`jj`)のcolocated構成です。変更の作成や履歴編集は`jj`で行い、GitHub連携は`jj git fetch` / `jj git push`経由で行います。`.jj/`はローカル専用でコミットしません。

```sh
jj status
jj log
jj commit -m "変更の要約"
jj bookmark move main --to @-
jj git push
```

エージェント向けの作業規約は[AGENTS.md](AGENTS.md)を見てください。

## 構成の要点

- 公開記事の正本はSupabase Postgres。埋め込み`articles/`は空DB時のseed用
- schemaは`supabase/migrations/`。mainへのpushでGitHub integrationが適用する。アプリは起動時にmigrateしない
- 管理: `/admin`(Supabase Auth passkey + `UNIGO_ADMIN_USER_IDS`)。読者ログインはX(Twitter) provider
- 画像はSupabase Storage公開バケット。Markdownの`/images/...`は公開ベースURLへ書き換え。アプリは`GET /images`を持たない
- 独自ドメインとTLSはVercelで終端し、External RewriteでCloud Runへ転送。アプリはHTTPのみ(`PORT`)

## License

[MIT License](LICENSE)
