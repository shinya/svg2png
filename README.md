# SVG2PNG

Pure Go で実装された SVG から PNG への変換ライブラリ。CGO 不使用でクロスプラットフォームに対応し、`qlmanage` 相当の変換品質を目指しています。

## 特徴

- **Pure Go 実装**: CGO なしでクロスプラットフォーム対応
- **高品質レンダリング**: `golang.org/x/image/vector` によるアンチエイリアス描画
- **完全なパスサポート**: M/L/H/V/C/S/Q/T/A/Z 全コマンド対応、楕円弧も Bezier 近似で正確に描画
- **グラデーション対応**: `linearGradient` / `radialGradient`（objectBoundingBox・userSpaceOnUse 両対応）
- **パターン対応**: `<pattern>` によるタイル塗り
- **クリッピング対応**: `clip-path` / `<clipPath>` による任意形状のクリッピング
- **破線対応**: `stroke-dasharray` / `stroke-dashoffset` による破線描画（直線・ポリライン・円・楕円・パス）
- **テキスト対応**: システムフォント自動検出、`text-anchor`、`tspan` 混合テキスト、`letter-spacing`
- **スタイル完全対応**: CSS インラインスタイル、プレゼンテーション属性、`fill: none` などを正確に処理
- **決定性**: 同一入力に対して常に同一の出力を保証
- **スレッドセーフ**: グローバルフォントマネージャーは `sync.RWMutex` で保護

## 対応要素

| カテゴリ | 要素・機能 |
|---|---|
| 図形 | `<rect>`（角丸対応）, `<circle>`, `<ellipse>`, `<line>`, `<path>`, `<polyline>`, `<polygon>` |
| テキスト | `<text>`, `<tspan>`（混合テキスト・インラインカラー変更）|
| グループ | `<g>`（子要素を再帰描画、`clip-path` 対応） |
| グラデーション | `<linearGradient>`, `<radialGradient>`（`objectBoundingBox` / `userSpaceOnUse`） |
| パターン | `<pattern>`（タイル繰り返し） |
| クリッピング | `<clipPath>`（polygon / rect / circle / path による任意形状） |
| スタイル | `fill`, `stroke`, `stroke-width`, `stroke-dasharray`, `stroke-dashoffset`, `opacity`, `fill-opacity`, `stroke-opacity`, `clip-path`, `font-family`, `font-size`（単位付き対応）, `font-style`, `font-weight`, `text-anchor`, `letter-spacing` |
| 色形式 | 名前付き色（CSS Color Level 4 準拠・150色以上）, `#RGB`, `#RRGGBB`, `#RGBA`, `#RRGGBBAA`, `rgb()`, `rgba()` |
| 単位 | `px`, `pt`, `em` |

## インストール

```bash
go get github.com/shinya/svg2png
```

## 基本的な使用方法

```go
package main

import (
    "fmt"
    "os"

    svg2png "github.com/shinya/svg2png/pkg/svg2png"
)

func main() {
    svgData, _ := os.ReadFile("input.svg")

    opts := svg2png.Options{
        Width:          800,
        Height:         600,
        DPI:            96,
        SystemFontScan: true, // システムフォントを自動検出
    }

    pngData, diag, err := svg2png.RenderPNG(svgData, opts)
    if err != nil {
        panic(err)
    }

    for _, w := range diag.Warnings {
        fmt.Println("Warning:", w)
    }

    os.WriteFile("output.png", pngData, 0644)
}
```

## フォント登録

```go
// システムフォントスキャンはデフォルトでON（何も指定不要）
opts := svg2png.Options{
    Width:  800,
    Height: 600,
}

// システムフォントスキャンを明示的に無効化したい場合のみ指定
opts := svg2png.Options{
    DisableSystemFontScan: true,
}

// カスタムフォントをファイルから登録
svg2png.RegisterFonts(svg2png.FontSource{
    Family: "Custom Font",
    Style:  "Regular", // Regular / Bold / Italic / BoldItalic
    Path:   "/path/to/font.ttf",
})

// メモリ上のフォントデータから登録
svg2png.RegisterFonts(svg2png.FontSource{
    Family: "In-Memory Font",
    Style:  "Bold",
    Data:   fontData, // []byte
})
```

## コマンドライン使用

```bash
# ビルド
go build ./cmd/svgpng

# 基本的な変換（システムフォントスキャンはデフォルトでON）
svgpng -in input.svg -out output.png -w 800 -h 600

# 背景色指定
svgpng -in input.svg -out output.png -w 800 -h 600 -bg "#ffffff"

# 透過背景
svgpng -in input.svg -out output.png -w 800 -h 600 -bg transparent

# システムフォントスキャンを無効にしたい場合のみ明示指定
svgpng -in input.svg -out output.png -w 800 -h 600 -no-system-font-scan
```

## 対応プラットフォーム

| OS | フォントパス |
|---|---|
| macOS | `/System/Library/Fonts`, `/Library/Fonts`, `~/Library/Fonts` |
| Linux | `/usr/share/fonts`, `/usr/local/share/fonts`, `~/.local/share/fonts` |
| Windows | `%WINDIR%/Fonts` |

## 制限事項

- `transform` 属性（`translate`, `rotate` など）は未対応
- SVG フィルタ（`<filter>`, `feGaussianBlur` など）は未対応
- `<use>` 要素による参照は未対応
- 外部リソース（URL 参照、外部 CSS）は未対応
- 絵文字・縦書き・`<textPath>` は未対応
