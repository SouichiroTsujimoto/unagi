# unagi

個人用のミニマルな技術ブログ。Goワンバイナリ(Echo / templ / is-land / Preact / SQLite / CertMagic)。

[unigo-template](https://github.com/SouichiroTsujimoto/unigo-template)から作成しています。

設計の不変条件は[AGENTS.md](AGENTS.md)。Nanos本番は[deploy/nanos/README.md](deploy/nanos/README.md)。

## 開発

```sh
cp .env.example .env
# 初回 /admin/setup 用に UNIGO_BOOTSTRAP_TOKEN_HASH を設定
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

- 公開記事の正本はSQLite。埋め込み`articles/`は空DB時のseed用
- 管理: `/admin`(passkey)。画像はlocalまたはGCS、配信は`/images/*`
- WebAuthn / sessionなどの秘密は環境変数。RP IDとoriginsは未設定時`[site].base_url`から決まる

## License

[MIT License](LICENSE)
