# images

Zenn互換の記事画像置き場です。Markdownからは `/images/example.png` のように参照します。

## 色付きBrailleアート(実験)

TUIバナーと同じ truecolor Braille を記事サムネ/ヘッダに使う場合:

1. 画像から生成する

```sh
go run ./cmd/braillegen path/to/image.png images/<slug>-ascii.txt 28
```

幅を大きくしたい例: `40` / `56` / `72`(比較用サンプルは `images/compare-w*-ascii.txt`)。

2. ファイル名を `images/<slug>-ascii.txt` にする(例: slug `hello-unagi` → `images/hello-unagi-ascii.txt`)

なければTUIの `logo-ascii.txt` 全体が使われます(クロップしません)。
