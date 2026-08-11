# unigo-template

Goのワンバイナリフルスタックwebアプリケーションのテンプレートです

- **ワンバイナリ**: 別ランタイム不要
- **デプロイ**: systemd、distrolessコンテナ、Nanosユニカーネルなど
- **開発者体験**: `just run`でホットリロード・TUI対応
- **Islands Architecture**: templ + `<is-land>` + Preact/htm (ビルド時・実行時 Node.js不要)

設計上の不変条件は[AGENTS.md](AGENTS.md)にあります。

![ホーム画面](docs/images/home-w.png)

![開発用TUI](docs/images/dev-tui.png)

## スタック

- **Go**
- **Echo**: HTTPルーティング
- **templ**: HTMLテンプレート
- **is-land + Preact/htm**: IslandsのリアクティブUI
- **Tailwind CSS + daisyUI**: Mondern CSS (Standalone CLI)
- **SQLite**: modernc.org/sqlite (CGO不要)
- **Bun**: SQL-first ORM
- **CertMagic**: 証明書取得とHTTPS
- **lipgloss / tint / huh / Bubble Tea**: 開発用TUI / ログ / initウィザード
- **just**: タスクランナー

## 始め方

タスクランナーとして[just](https://github.com/casey/just#installation)を使用します。GitHubで**Template repository**から作成したあと:

```sh
just init
cp .env.example .env   # 必要なら UNIGO_BOOTSTRAP_TOKEN_HASH を設定
just run
```

既定では`http://localhost:8080`で起動します。

`just run`(cmd/dev)はプロジェクトルートの`.env`を読みます(未設定の環境変数のみ)。`bin/server`は読みません。秘密情報は`.unigo.toml`ではなく`.env`か環境変数へ置いてください。

`just init`は次を行います(Charmのhuh)。

1. Go module pathの設定(プロジェクト全体を置換)
2. Tailwind CSS + daisyUIのStandalone CLIを`tools/`へ入れる(既存ならスキップ)

### 開発時のオプション

```sh
just run                 # logoあり + TUI(既定)
just run logo=false      # logoなし + TUI
just run tui=false       # logoあり + 素のログ流し
just run logo=false tui=false
```

- `tui=true`のときはBubble Teaの開発用TUIを起動します。アプリログは`logs`へ、Air / css-watchの出力は`air`パネルへ分離します。
- `tui=false`(またはTTYでない場合)は素のAir起動になります。

`bin/server`ではTUIは起動しません。

既定値は`.unigo.toml`の
- `[dev]` 
- `[db]` 
- `[banner]`
で設定します。

カスタムのlistenバナーロゴは`[banner].image`に画像パスを書き、`just logo`(または`just run`時の自動生成)で`<name>-ascii.txt`を作ります。生成物と`.sha256`はコミットしてください。

## ビルド

自分のマシンと同じOS/CPU向けの実行ファイルは次で作れます。

```sh
just build
bin/server
```

他のOSやコンテナ、ユニカーネルへ載せる手順は次の「デプロイ」にまとめています。

## 実行時設定

`bin/server`の主なフラグです。開発でも本番でも同じです。

```sh
# HTTP(開発やリバースプロキシの後ろ)
bin/server -addr :8080 -db app.db

# HTTPS(CertMagicが証明書を取得)
bin/server \
  -domain example.com \
  -email admin@example.com \
  -db-driver sqlite \
  -db app.db

# Postgresに切り替える例
# bin/server -db-driver postgres -db 'postgres://user:pass@localhost:5432/unigo?sslmode=require'
```

DBの既定はSQLiteです。`.unigo.toml`の`[db]`でも切り替えられます。

## デプロイ

成果物は静的なワンバイナリです。

| 方式 | 対象 | 手順 |
|------|----------------|----------------|
| **systemd** | Linuxサーバ / VM | ①バイナリ作成 → ②配置とunit |
| **distrolessコンテナ** | Docker | Dockerfileだけで完結(①は不要) |
| **Nanosユニカーネル** | 軽量VM | ①バイナリ作成 → ②opsで起動 |

SQLite(`*.db`)とCertMagic(`.certmagic/`など)はバイナリの外に置きます。永続化が要るときは書き込み可能なディスク／ボリュームをマウントしてください。

### ビルド(systemd / Nanos)

```sh
# x86_64のLinux VMなど
GOOS=linux GOARCH=amd64 just build

# arm64のLinux / Apple SiliconホストからNanosを試すときなど
GOOS=linux GOARCH=arm64 just build
```

成果物は`bin/server`です。

#### systemd

ビルドした`bin/server`をLinuxホストへコピーしたうえで:

```sh
sudo install -d -o www-data -g www-data /opt/unigo /var/lib/unigo
sudo install -m 755 bin/server /opt/unigo/server
```

`/etc/systemd/system/unigo.service`の例:

```ini
[Unit]
Description=unigo web server
After=network.target

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/var/lib/unigo
ExecStart=/opt/unigo/server -addr :8080 -db /var/lib/unigo/app.db
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now unigo
```

リバースプロキシの後ろなら`-addr 127.0.0.1:8080`にし、証明書はプロキシ側でも構いません。

#### distroless Docker

ホストでのクロスコンパイルは不要です。次の`Dockerfile`をリポジトリ直下に置き、イメージ内でLinuxバイナリをビルドします(コミット済みの`*_templ.go`と`static/app.css`を使います)。

```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /data
COPY --from=build /out/server /server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
CMD ["-addr", ":8080", "-db", "/data/app.db"]
```

```sh
docker build -t unigo:latest .
docker run --rm -p 8080:8080 -v unigo-data:/data unigo:latest
```

Cloud Runなどのサーバレス環境ではコンテナのエフェメラルFSにSQLiteを置かないでください。永続が必要ならPostgres(`-db-driver postgres`)やマウント可能なボリュームを使います。

### Nanosユニカーネル(ops)

[Nanos](https://nanos.org/)上で、Linuxユーザー空間なしに同じELFを動かします。実行には[ops](https://ops.city/)が必要です。

1. `bin/server`をビルド
2. `ops.json`を用意
3. 起動

本番ではボリュームをマウントしてDBを永続化してください。

`ops.json`の例:

```json
{
  "BaseVolumeSz": "200m",
  "Args": ["-addr=:8080", "-db=/data/app.db"],
  "Dirs": ["data"],
  "Env": {
    "HOME": "/tmp",
    "TMPDIR": "/tmp"
  },
  "RunConfig": {
    "Ports": ["8080"]
  }
}
```

```sh
mkdir -p data
ops run bin/server -c ops.json -p 8080 -m 512M
# -m 512M: 既定ボリュームだとSQLiteが`database or disk is full`になりやすい
```

永続化やクラウド向けの詳細は[opsのドキュメント](https://docs.ops.city/ops/configuration)を参照してください。

## アーキテクチャ

### Islands

ドキュメントシェルはtempl、状態と再描画はislandのESMに閉じます。

- islandのソースは`internal/web/islands`
- 依存(is-land / Preact / htm / preact-custom-element)は`static/vendor`にvendoring(再取得は`just vendor-islands`)

サンプルの`account-panel`が呼ぶAPI:

```http
GET    /api/accounts
POST   /api/accounts
DELETE /api/accounts/:id
```

## 機能の追加

1. `internal/feature/<name>`へ機能を追加する
2. 必要なschemaを`internal/db/migrations/<driver>`へ追加する
3. `internal/web/<area>`へhandlerとtemplを追加する(islandが必要なら`internal/web/islands`も)
4. `internal/app/app.go`で依存関係を組み立てる
5. `internal/web/router.go`へrouteを登録する

詳細な不変条件は[AGENTS.md](AGENTS.md)を見てください。

## ディレクトリ構成

```text
.
├── cmd/
│   ├── server/main.go
│   ├── dev/                # just run(開発ランチャー / TUI)
│   ├── logo/               # just logo(バナーASCII生成)
│   └── init/               # just init(テンプレート初期化)
├── internal/
│   ├── feature/
│   │   └── account/        # 業務APIの最小例
│   ├── app/                # 依存関係の組み立て
│   ├── config/
│   ├── httpserver/
│   ├── db/
│   ├── terminal/
│   ├── logogen/
│   └── web/                # Echo / templ / islands
├── static/                 # 埋め込み資産(CSS、vendor)
├── tools/                  # Tailwind Standalone CLI(gitignore)
├── .unigo.toml
├── .air.toml
├── justfile
└── go.mod
```

`*_templ.go`は生成物です。直接編集せず、対応する`.templ`を編集します。このリポジトリでは生成ファイルをコミットし、CIで再生成差分がないことを確認します。

## 検証

```sh
go test ./...
go vet ./...
just build
```

## License

[MIT License](LICENSE)
