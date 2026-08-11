---
title: "【Astrobit】MoonBitでAstroのコンポーネントを書いてみた"
emoji: "🌔"
type: "tech"
topics: ["typescript", "astro", "moonbit"]
published: true
published_at: 2026-03-25 17:34
---
Astroにはisland機能でReactやSvelteなどで書いたcomponentが使用できるように公式でインテグレーションが用意されているのですが、その他のUIフレームワークでもcomponentを書くためにカスタムインテグレーションが作成できるようになっています。

そこで、moonbitのコードでastroのcomponentを記述するUIフレームワーク**Astrobit**を作成しました。リアクティブな動作はmizchiさんの [signals](https://mooncakes.io/docs/mizchi/signals) を用いています。これによりsolid.jsのようなsignalベースのDOM更新が可能になっています。

- パッケージ: https://mooncakes.io/docs/SouichiroTsujimoto/astrobit

- 実際に使用したサンプルページ: https://astrobit-sample.vercel.app/

## 使い方(例)

具体的には[astrobit-sample](https://astrobit-sample.vercel.app/)の[リポジトリ](https://github.com/SouichiroTsujimoto/astrobit-sample)のREADMEや、[astrobit本体](https://mooncakes.io/docs/SouichiroTsujimoto/astrobit)を参照してください。

環境構築は(結構簡単にできるようになっているつもりなので)省略して、コア部分を紹介します

`src/components/counter.mbt`などに以下のようにコンポーネントを書きます

```Rust
fn counter(props : @dom.Props) -> @a.Node {
  let initial = props.get_int("initial")
  let count = @signals.signal(initial)
  @a.div(attrs={ "class": "counter-widget" }, [
    @a.div(attrs={ "class": "counter-display" }, [
      @a.span(
        attrs={ "class": "counter-value" },
        @a.dyn_text(fn() { count.get().to_string() }),
      ),
    ]),
    @a.div(attrs={ "class": "counter-controls" }, [
      @a.button(attrs={ "class": "counter-btn btn-dec" }, "−")
      |> @a.on_click(fn(_) { count.update(fn(n) { n - 1 }) }),
      @a.button(attrs={ "class": "counter-btn btn-reset" }, "Reset")
      |> @a.on_click(fn(_) { count.set(initial) }),
      @a.button(attrs={ "class": "counter-btn btn-inc" }, "+")
      |> @a.on_click(fn(_) { count.update(fn(n) { n + 1 }) }),
    ]),
  ])
}
```

```Rust
///|
pub fn mount(element : @dom.Element, props : @dom.Props) -> Unit {
  @a.mount_dom(element, counter(props))
}

///|
pub fn render(props : @dom.Props) -> String {
  @a.render_to_html(counter(props))
}

///|
pub fn hydrate(element : @dom.Element, props : @dom.Props) -> Unit {
  @a.hydrate_dom(element, counter(props))
}
```

あとは`src/components/Welcome.astro`などのファイルから

```tsx
import Counter from './counter/counter.mbt'

// ...

<Counter client:load initial={0} />
```

このように書くだけで使用できます

ディレクトリ構造はこんな感じです

```
src/
├── assets
│   ├── astro.svg
│   └── background.svg
├── components
│   ├── Welcome.astro
│   ├── codeSnippets.ts
│   ├── counter
│   │   ├── counter.mbt
│   │   └── moon.pkg
│   └── todos
│       ├── moon.pkg
│       └── todos.mbt
├── layouts
│   └── Layout.astro
└── pages
```


## Moonbitもっと使いたい

個人的にはwasm-gcをターゲットとしたビルドに惹かれてMoonbitを触ってみたので、wasmを使用した面白い例も書いてみたいです。Astrobitの前に、spin frameworkを使用したwasmで動作するAPIサーバをClaudeに書かせたりしたのですが、こちらももう少しちゃんと公開する価値のあるものになれば公開したいです。
