# unagi

個人用のミニマルな技術ブログ。Goワンバイナリ(Echo / templ / is-land / Preact) + Supabase(Postgres / Auth / Storage) + Vercel(`Dockerfile.vercel`、東京)。

[unigo-template](https://github.com/SouichiroTsujimoto/unigo-template)から作成しています。

設計の不変条件は[AGENTS.md](AGENTS.md)。本番手順は[deploy/README.md](deploy/README.md)。

## 開発

```sh
cp .env.example .env
supabase gen signing-key --algorithm ES256 > supabase/signing_keys.json
supabase start
just run
# /admin は初回Xログイン後、UsersのUUIDを UNIGO_ADMIN_USER_IDS へ
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
- 管理: `/admin`(Supabase AuthのX OAuth + `UNIGO_ADMIN_USER_IDS`)。読者ログインも同じX provider
- 画像はSupabase Storage公開バケット。管理画面はsigned URLで直接uploadし、Markdownの`/images/...`は公開ベースURLへ書き換え。アプリは`GET /images`を持たない
- 独自ドメインとTLSはVercelで終端し、同じprojectの`Dockerfile.vercel`コンテナ(東京`hnd1`)がアプリを動かす。アプリはHTTPのみ(`PORT`)

## License

[MIT License](LICENSE)
