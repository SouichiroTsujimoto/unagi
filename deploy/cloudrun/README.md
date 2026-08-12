# Cloud Run

東京(`asia-northeast1`)、min-instances=0、リクエスト課金。TLSはCloud Runのカスタムドメインマッピング(マネージド証明書)で、アプリはHTTPだけを話し`PORT`を読む。DB / Auth / StorageはSupabase、画像はStorageから直接配信する。

`deploy/cloudrun/`のスクリプトは`config.sh`で設定を共有し、`justfile`と`.github/workflows/ci.yml`の両方から同じものが呼ばれる。手動とCDで経路が分かれない。

## 日常

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

埋めるのは`GCP_PROJECT`、`GITHUB_REPO`、`UNIGO_SITE_BASE_URL`、`UNIGO_MEDIA_PUBLIC_BASE`。残りは既定値でよい。秘密の値はここに書かない。書くのはSecret Managerの名前だけで、既定名を使うなら何も書かなくていい。

### 2. Supabase

画面での作業なので自動化しない。

1. プロジェクトを`ap-northeast-1`で作る。
2. DashboardのIntegrationsでGitHubをこのリポジトリに繋ぐ。Working directoryは`.`。**Deploy to productionをON**にする。Automatic branchingは使わない。
3. Authでpasskeyを有効にし、X(Twitter) providerにclient IDとsecretを入れる。secretはアプリではなくSupabase側に置く。
4. AuthのSite URLとredirect URLに`UNIGO_SITE_BASE_URL`と`{BASE}/auth/x/callback`を入れる。
5. Studioで管理者ユーザを作り、UUIDを控える。ここに入れた人だけが`/admin`に入れる。
6. 接続文字列とキーを控える。次の手順でSecret Managerに入れる。

ローカルは`supabase start`が同じ`supabase/migrations/`を流す。既存のローカルDBをbun時代から引き継いでいる場合は`supabase db reset`。

### 3. GCPとGitHubの構築

```sh
just cloudrun-bootstrap
```

API有効化、Artifact Registry、サービスアカウント2つ(ランタイム用と、GitHub Actionsが借りるデプロイ用)、Secret Manager 6件と参照権限、GitHub Actions向けのWorkload Identity Federation、CDが読むリポジトリ変数を作る。何度実行してもよい。

長期鍵は作らない。GitHubに置くのは非秘密の変数だけで、アプリの秘密はSecret Managerから離れない。

### 4. 秘密の値を入れる

`cloudrun-bootstrap`が作るのは空のsecretなので、値は別に入れる。プロンプトは表示されず、シェル履歴にもファイルにも残らない。

```sh
just cloudrun-secret            # 名前の一覧を表示
just cloudrun-secret unagi-db-dsn
```

`UNIGO_DB_DSN`はpoolerのsession mode(ポート5432)を使う。直接接続(`db.<ref>.supabase.co`)はIPv6のみでCloud Runの下り経路から届かず、transaction mode(6543)はprepared statementと衝突する。

### 5. カスタムドメイン

```sh
gcloud beta run domain-mappings create --service=unagi --domain=example.com \
  --region=asia-northeast1 --project="${GCP_PROJECT}"
```

表示されたレコードをDNSに入れる。証明書の発行までは数分から数時間かかる。

反映したら`.env`の`UNIGO_SITE_BASE_URL`を直し、`just cloudrun-bootstrap`(リポジトリ変数の更新)と`just cloudrun-deploy`をやり直す。SupabaseのSite URLとredirect URLも同じ値に揃える。**ここがずれるとOAuth callbackとOriginチェックが落ちる。**症状はログインだけが失敗する形で出るので、疑う場所として覚えておく。

## 性質

- min-instances=0なので、間が空いた後の初回リクエストはコールドスタートになる。
- Supabaseの無料枠は7日無アクセスでpauseし、その間はDB接続が落ちる。
- Cloud Runのデプロイとmigration適用は同じpushで並行する。初回だけ、schemaが先に着く前に起動すると失敗するので、Actionsを再実行する。
