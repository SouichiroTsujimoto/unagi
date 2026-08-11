# Nanos / GCE 運用メモ

単一Nanosインスタンス + Persistent Disk(`/data`) + (任意)GCS 画像bucket構成です。

## ローカルでの永続化パス

| 用途 | 既定パス |
|------|----------|
| SQLite | `-db /data/unagi.db` |
| CertMagic | `UNIGO_CERTMAGIC_STORAGE=/data/certmagic` |
| ローカル画像 | `UNIGO_MEDIA_DIR=/data/media` |

## 必要な環境変数

```sh
# サイト
# .unigo.toml の [site] も利用可

# Passkey (必須)
export UNIGO_WEBAUTHN_RPID=example.com
export UNIGO_WEBAUTHN_ORIGINS=https://example.com
export UNIGO_SECURE_COOKIES=true

# 初回セットアップ用(最初のpasskey登録後は不要)
TOKEN=$(openssl rand -hex 16)
echo "bootstrap token: $TOKEN"
export UNIGO_BOOTSTRAP_TOKEN_HASH=$(printf '%s' "$TOKEN" | shasum -a 256 | awk '{print $1}')

# 画像
export UNIGO_MEDIA_BACKEND=gcs   # または local
export UNIGO_GCS_BUCKET=unagi-media
export UNIGO_GCS_PREFIX=images
```

GCE上ではApplication Default Credentials(サービスアカウントのmetadata)を使い、key JSONはイメージへ入れません。

## ops.json 例

```json
{
  "BaseVolumeSz": "500m",
  "Args": [
    "-addr=:443",
    "-db=/data/unagi.db",
    "-domain=example.com",
    "-email=admin@example.com"
  ],
  "Env": {
    "HOME": "/tmp",
    "TMPDIR": "/tmp",
    "UNIGO_CERTMAGIC_STORAGE": "/data/certmagic",
    "UNIGO_MEDIA_BACKEND": "gcs",
    "UNIGO_GCS_BUCKET": "unagi-media",
    "UNIGO_GCS_PREFIX": "images",
    "UNIGO_WEBAUTHN_RPID": "example.com",
    "UNIGO_WEBAUTHN_ORIGINS": "https://example.com",
    "UNIGO_SECURE_COOKIES": "true",
    "UNIGO_BOOTSTRAP_TOKEN_HASH": "REPLACE_WITH_SHA256"
  },
  "RunConfig": {
    "Klibs": ["tls", "gcp"],
    "Ports": ["80", "443"]
  },
  "CloudConfig": {
    "ProjectID": "YOUR_PROJECT",
    "Zone": "asia-northeast1-a",
    "BucketName": "YOUR_OPS_ARTIFACT_BUCKET",
    "InstanceProfile": "unagi-runtime"
  },
  "Mounts": {
    "unagi-data": "/data"
  }
}
```

## デプロイ手順(要約)

1. `GOOS=linux GOARCH=amd64 just build`
2. GCS media bucketを作成し、versioningを有効化
3. runtimeサービスアカウントへ対象bucketの`roles/storage.objectAdmin`(またはより狭いカスタムrole)を付与
4. Persistent Diskを作成しsnapshot scheduleを設定
5. `ops image create` → `ops instance create`、static IPとfirewall(80/443)を設定
6. DNS Aレコードをstatic IPへ向ける
7. `/admin/setup`でbootstrap token + 1Password passkeyを登録し、recovery codesを保存
8. 予備passkeyを追加

## 更新手順(単一インスタンス)

1. インスタンスを停止
2. data diskをdetach
3. 新imageでインスタンス作成
4. 同じdata diskを`/data`へattach
5. health確認(`/`, `/admin/login`, 記事表示)

## バックアップ

- Persistent Disk: GCP snapshot schedule(日次)
- GCS media bucket: object versioning + lifecycle
- 必要なら定期的にSQLiteファイルをGCSへコピー
