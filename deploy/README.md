# 本番(Vercel container)

東京(`hnd1`)のVercel Functions上で、rootの`Dockerfile.vercel`が作るdistroless Goバイナリを動かす。独自ドメインとTLSも同じVercel projectが終端する。アプリはHTTPだけを話し`PORT`を読む。DB / Auth / StorageはSupabase、画像はStorageへbrowserが直接PUTし、公開URLから直接配信する。

Vercel FunctionやMiddleware、External Rewriteは使わない。

## 運用

**mainへpushすると本番が更新される。** Vercel Git integrationが`Dockerfile.vercel`をbuildしてdeployし、[Supabase GitHub integration](https://supabase.com/docs/guides/deployment/branching/github-integration)のDeploy to productionが`supabase/migrations/`を適用する。アプリは起動時にmigrateしない。GitHub Actionsのverify jobはテストするだけで、deployの必須gateにはしない。

戻すときはrevertしてpushする。schemaの巻き戻しも新しいmigrationファイルを足す(適用済みファイルは書き換えない)。

## 初回だけ

### 1. Supabase

画面での作業なので自動化しない。

1. プロジェクトを`ap-northeast-1`で作る。
2. DashboardのIntegrationsでGitHubをこのリポジトリに繋ぐ。Working directoryは`.`。**Deploy to productionをON**にする。Automatic branchingは使わない。
3. Authでpasskeyを有効にし、X(Twitter) providerにclient IDとsecretを入れる。secretはアプリではなくSupabase側に置く。
4. AuthのSite URLとredirect URLに`UNIGO_SITE_BASE_URL`と`{BASE}/auth/x/callback`を入れる。
5. Studioで管理者ユーザを作り、UUIDを控える。ここに入れた人だけが`/admin`に入れる。
6. 接続文字列と、Settings > API Keysのpublishable / secretを控える。Legacyのanon / service_roleは使わない。JWTは`/auth/v1/.well-known/jwks.json`で検証するのでJWT secretは不要。

`UNIGO_DB_DSN`はpoolerのsession mode(ポート5432)を使う。直接接続(`db.<ref>.supabase.co`)はIPv6のみで届かないことがあり、transaction mode(6543)はprepared statementと衝突する。

ローカルは`supabase start`が同じ`supabase/migrations/`を流す。既存のローカルDBをbun時代から引き継いでいる場合は`supabase db reset`。

画像uploadはGoがsigned URLを発行し、browserがSupabase Storageへ直接PUTする。Storage CORSでサイトorigin(`UNIGO_SITE_BASE_URL`とローカルの`http://localhost:8080`)を許可する。DashboardのStorage settingsから設定する。

### 2. Vercel projectと環境変数

このGitHub repositoryをVercelへimportする。値はGitHubやリポジトリのファイルには置かない。Vercel Dashboardの **Project → Settings → Environment Variables** へ入れる。

1. Root Directoryはリポジトリroot(`.`)。`deploy/vercel`ではない。
2. Framework PresetはOther。`Dockerfile.vercel`があればVercelがcontainerとして検出する。
3. Function RegionをTokyo (`hnd1`)にする。`vercel.json`の`regions`も同じ値。
4. 下表を **Production** に入れる。Sensitiveな値はSensitiveにする。import直後に自動deployが走っても、変数を入れてから **Deployments → Redeploy** すれば反映される。
5. Domainsへ`unagi.wuhu1s.land`を追加する。`UNIGO_SITE_BASE_URL`は最初からこのURLで入れてよい。

値の入手元はSupabase Dashboard(手順1で控えたもの)。手元に`deploy/cloudrun/.env`が残っていれば、同じ`UNIGO_*`をコピーしてよい。

| 変数 | 値 | Sensitive |
| --- | --- | --- |
| `UNIGO_DB_DSN` | pooler session mode(5432) | yes |
| `UNIGO_SUPABASE_URL` | `https://YOUR_REF.supabase.co` | |
| `UNIGO_SUPABASE_PUBLISHABLE_KEY` | `sb_publishable_...` | |
| `UNIGO_SUPABASE_SECRET_KEY` | `sb_secret_...` | yes |
| `UNIGO_ADMIN_USER_IDS` | 管理者UUID(カンマ区切り) | yes |
| `UNIGO_SITE_BASE_URL` | `https://unagi.wuhu1s.land` | |
| `UNIGO_MEDIA_PUBLIC_BASE` | `https://YOUR_REF.supabase.co/storage/v1/object/public/images` | |
| `UNIGO_SITE_NAME` | 任意 | |
| `UNIGO_SITE_DESCRIPTION` | 任意 | |
| `PORT` | `8080`(`Dockerfile.vercel`の既定と同じ) | |

`CLOUD_RUN_ORIGIN`は使わない。GitHub Actionsのrepository variablesにも入れない。

`UNIGO_SITE_BASE_URL`をこのあと変えた場合は、Vercelの環境変数とSupabaseのSite URL / redirect URLを同じ値に揃えて再deployする。**ここがずれるとOAuth callbackとOriginチェックが落ちる。**症状はログインだけが失敗する形で出る。

Vercel Hobbyは個人・非商用だけに使える。

## 性質

- idle後の初回リクエストはコールドスタートになる。
- 画像本体はVercelを通らない。Goは5 MiBまでの検証だけを行う。
- Supabaseの無料枠は7日無アクセスでpauseし、その間はDB接続が落ちる。
- Vercelのデプロイとmigration適用は同じpushで並行する。初回だけ、schemaが先に着く前に起動すると失敗するので、Vercel側を再deployする。

## 旧GCPリソースの削除

Cloud Run経路は使わない。以前`just cloudrun-bootstrap`を流していれば、GCP側に残骸が残っている。Dashboardまたは`gcloud`で次を消せる。

1. Cloud Run service(`unagi`、`asia-northeast1`)
2. Artifact Registry repository(`unagi`)
3. Secret Managerの`unagi-*` secret
4. Workload Identity Federation pool / provider
5. サービスアカウント(`unagi-run`、`unagi-deployer`)
6. GitHub repository variables(`GCP_*`、`CLOUD_RUN_*`、`ARTIFACT_REPO`、`RUNTIME_SA`、`UNIGO_*_SECRET`)

未作成ならこの手順は不要。
