# Cloud Run

東京(`asia-northeast1`)、min-instances=0、リクエスト課金。独自ドメインとTLSはVercelが終端し、External RewriteでCloud Runへ転送する。アプリはHTTPだけを話し`PORT`を読む。DB / Auth / StorageはSupabase、画像はStorageから直接配信する。

`deploy/cloudrun/`のスクリプトは`config.sh`で設定を共有し、`justfile`と`.github/workflows/ci.yml`の両方から同じものが呼ばれる。手動とCDで経路が分かれない。

## 運用

**mainへpushすると本番が更新される。** GitHub Actionsがテストを通してからCloud Runへデプロイし、[Supabase GitHub integration](https://supabase.com/docs/guides/deployment/branching/github-integration)のDeploy to productionが`supabase/migrations/`を適用する。アプリは起動時にmigrateしない。

戻すときはrevertしてpushする。schemaの巻き戻しも新しいmigrationファイルを足す(適用済みファイルは書き換えない)。

手元から流す必要があるときだけ、次を使う。

```sh
just cloudrun-release   # build + push + deploy
just cloudrun-deploy    # イメージはそのままに、env varの変更だけ反映
just cloudrun-logs
```

## 初回だけ

### 1. `deploy/cloudrun/.env`

すべてのスクリプトがこれを読む。無いと`set GCP_PROJECT`で止まる。gitignore済み。

```sh
cp deploy/cloudrun/env.example deploy/cloudrun/.env
```

埋めるのは`GCP_PROJECT`、`GITHUB_REPO`、`UNIGO_SITE_BASE_URL`、`UNIGO_MEDIA_PUBLIC_BASE`、および手順4の秘密5件。残りは既定値でよい。Secret Managerの名前は既定のままでよい。このファイルはgitignore済み。

### 2. Supabase

画面での作業なので自動化しない。

1. プロジェクトを`ap-northeast-1`で作る。
2. DashboardのIntegrationsでGitHubをこのリポジトリに繋ぐ。Working directoryは`.`。**Deploy to productionをON**にする。Automatic branchingは使わない。
3. Authでpasskeyを有効にし、X(Twitter) providerにclient IDとsecretを入れる。secretはアプリではなくSupabase側に置く。
4. AuthのSite URLとredirect URLに`UNIGO_SITE_BASE_URL`と`{BASE}/auth/x/callback`を入れる。
5. Studioで管理者ユーザを作り、UUIDを控える。ここに入れた人だけが`/admin`に入れる。
6. 接続文字列と、Settings > API Keysのpublishable / secretを控える。Legacyのanon / service_roleは使わない。JWTは`/auth/v1/.well-known/jwks.json`で検証するのでJWT secretは不要。次の手順でSecret Managerに入れる。

ローカルは`supabase start`が同じ`supabase/migrations/`を流す。既存のローカルDBをbun時代から引き継いでいる場合は`supabase db reset`。

### 3. GCPとGitHubの構築

```sh
just cloudrun-bootstrap
```

API有効化、Artifact Registry、サービスアカウント2つ(ランタイム用と、GitHub Actionsが借りるデプロイ用)、Secret Manager 5件と参照権限、GitHub Actions向けのWorkload Identity Federation、CDが読むリポジトリ変数を作る。何度実行してもよい。

長期鍵は作らない。GitHubに置くのは非秘密の変数だけで、アプリの秘密はSecret Managerから離れない。

### 4. 秘密の値を入れる

`cloudrun-bootstrap`が作るのは空のsecretなので、値は`deploy/cloudrun/.env`に書いてから一度流す。

```sh
just cloudrun-secret
```

`UNIGO_DB_DSN`はpoolerのsession mode(ポート5432)を使う。直接接続(`db.<ref>.supabase.co`)はIPv6のみでCloud Runの下り経路から届かず、transaction mode(6543)はprepared statementと衝突する。スペースやシェルの特殊文字を含む値は単引用符で囲む。

### 5. Vercelの独自ドメイン

初回デプロイ(`just cloudrun-release`またはCI)のあと、Cloud Runのorigin URLを確認する。

```sh
bash -c 'source deploy/cloudrun/config.sh
gcloud run services describe "${CLOUD_RUN_SERVICE}" \
  --project="${GCP_PROJECT}" \
  --region="${GCP_REGION}" \
  --format="value(status.url)"'
```

VercelでこのGitHub repositoryを別projectとしてimportし、次を設定する。

1. Root Directoryを`deploy/vercel`にする。
2. Framework PresetをOtherにする。
3. ProductionとPreviewの環境変数`CLOUD_RUN_ORIGIN`に、上で得たURLを末尾の`/`なしで入れる。
4. deploy後、projectのDomainsへ`unagi.wuhu1s.land`を追加する。

`deploy/vercel/vercel.json`が全pathをCloud Runへ転送する。Vercel FunctionやMiddlewareは使わず、初期状態ではExternal Rewriteのcacheも無効にする。Cloud Runは`--allow-unauthenticated`のままなので、`run.app`のURLへ直接アクセスする経路も残る。

Vercel Hobbyは個人・非商用だけに使える。目安は月100GBのFast Data Transferと100万Edge Requestsで、画像閲覧はSupabase Storageから直接配信される。広告、寄付、収益目的を追加した場合は通信量にかかわらずProへ移行する。

`UNIGO_SITE_BASE_URL`をこのあと変えた場合だけ、`just cloudrun-bootstrap`と`just cloudrun-deploy`をやり直す。SupabaseのSite URLとredirect URLも同じ値に揃える。**ここがずれるとOAuth callbackとOriginチェックが落ちる。**症状はログインだけが失敗する形で出るので、疑う場所として覚えておく。

## 性質

- min-instances=0なので、間が空いた後の初回リクエストはコールドスタートになる。
- VercelのExternal Rewriteは120秒でtimeoutする。
- Supabaseの無料枠は7日無アクセスでpauseし、その間はDB接続が落ちる。
- Cloud Runのデプロイとmigration適用は同じpushで並行する。初回だけ、schemaが先に着く前に起動すると失敗するので、Actionsを再実行する。
