# 記事リポジトリ同期

記事と画像の正本は[SouichiroTsujimoto/unagi-content](https://github.com/SouichiroTsujimoto/unagi-content)です。Postgresは公開サイトのread modelです。

## 接続手順

1. このアプリをdeployし、VercelのSensitive Environment Variableへ次を入れる。
   - `UNIGO_CONTENT_SYNC_SECRET`(長い乱数)
2. 記事リポジトリのActions secrets/variablesへ同じ値を入れる。
   - secret: `UNIGO_CONTENT_SYNC_SECRET`
   - variable: `UNIGO_SITE_BASE_URL=https://unagi.wuhu1s.land`
3. 既存記事・画像を記事リポジトリの`main`へpushする。
4. `sync` workflowが不足画像をStorageへPUTし、全量snapshotを`/api/content-sync/sync`へ送る。
5. `/admin`で公開状態を確認する。既存slugは初回同期で公開状態を維持する。新規ファイルは下書きになる。

DB DSNやSupabase secret keyは記事リポジトリに置かない。
repository名はActions標準の`GITHUB_REPOSITORY`を使うため、追加variableは不要。

## ローカル同期

`supabase start`と`just run`を起動した状態で、アプリリポジトリから次を実行する。

```sh
just content-sync-local
```

初回は公開リポジトリ`SouichiroTsujimoto/unagi-content`を`../unagi-content`へcloneし、以後はその作業コピーを全量同期する。別の配置を使う場合は`just content-sync-local /path/to/unagi-content`とする。

ローカル同期は`cmd/dev`だけが持つ固定HMAC値を使う。本番の`UNIGO_CONTENT_SYNC_SECRET`は参照しない。新規記事は下書きになるため、同期後に`http://localhost:8080/admin`で確認対象の記事を公開する。記事を更新したい場合はcontent repository側でpullしてから再実行する。

## 契約

- frontmatterは`title`、`emoji`、`type`、`topics`だけ。`published` / `published_at`は使わない。
- slugは`articles/<slug>.md`のファイル名。公開後のrenameは禁止(renameすると旧slugは物理削除され、コメント・ステッカーは戻らない)。
- Gitから消えた記事はDBから物理削除する。cascadeでコメントとステッカーも消える。
- 画像は`images/`。同期時に`{sha256}.{ext}`へ変換してStorageへ置く。`images/`から消えたファイルに対応するオブジェクトはStorageからも削除する。
- 投稿日(`published_at`)はadminが初めて公開した時刻。本文同期・非公開・再公開では変えない。

## 復旧

同じcommitを`workflow_dispatch`で再実行してよい。`run_id`が新しければ再適用される。同じ`run_id`のリプレイは409になる。

署名はHMAC-SHA-256。canonical stringは次のとおり。

```
METHOD
PATH
UNIX_TIMESTAMP
RUN_ID
REPOSITORY
SHA256(body)
```

ヘッダは`X-Unigo-Timestamp`、`X-Unigo-Run-Id`、`X-Unigo-Repository`、`X-Unigo-Signature`(`sha256=<hex>`)。
