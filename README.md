# SVG2PNG

Pure Go で実装された SVG から PNG への変換ライブラリ。
現在作成中。品質はまだまだ。

## 特徴

- **Pure Go 実装**: CGO なしでクロスプラットフォーム対応
- **テキスト対応**: `<text>`要素の完全サポート、システムフォント自動検出
- **高品質レンダリング**: アンチエイリアス、透明度、変形対応
- **決定性**: 同一入力に対して常に同一の出力を保証
- **並列処理対応**: スレッドセーフな実装

## 対応要素

- **図形**: `<path>`, `<rect>`, `<circle>`
- **テキスト**: `<text>`, `<tspan>`（同一行内）
- **スタイル**: `fill`, `stroke`, `opacity`, `transform`
- **単位**: `px`, `%`（viewBox 基準）

## インストール

```bash
go get github.com/shinya/svg2png
```

## 基本的な使用方法

```go
package main

import (
    "github.com/shinya/svg2png"
)

func main() {
    svgData := []byte(`<svg><text x="10" y="20">Hello World</text></svg>`)

    opts := svg2png.Options{
        Width:  800,
        Height: 600,
        DPI:    96,
    }

    pngData, diag, err := svg2png.RenderPNG(svgData, opts)
    if err != nil {
        panic(err)
    }

    // 診断情報の確認
    for _, warning := range diag.Warnings {
        fmt.Println("Warning:", warning)
    }

    // PNGデータの保存
    os.WriteFile("output.png", pngData, 0644)
}
```

## フォント登録

```go
// システムフォントの自動検出（デフォルト）
svg2png.RenderPNG(svgData, opts)

// カスタムフォントの登録
svg2png.RegisterFonts(svg2png.FontSource{
    Family: "Custom Font",
    Style:  "Regular",
    Path:   "/path/to/font.ttf",
})

// メモリからの登録
svg2png.RegisterFonts(svg2png.FontSource{
    Family: "In-Memory Font",
    Style:  "Bold",
    Data:   fontData,
})
```

## コマンドライン使用

```bash
# 基本的な変換
svgpng -in input.svg -out output.png -w 800 -h 600

# 背景色指定
svgpng -in input.svg -out output.png -w 800 -h 600 -bg "#ffffff"

# 透過背景
svgpng -in input.svg -out output.png -w 800 -h 600 -bg transparent
```

## 対応プラットフォーム

- Linux: `/usr/share/fonts`, `/usr/local/share/fonts`
- macOS: `/System/Library/Fonts`, `/Library/Fonts`
- Windows: `%WINDIR%/Fonts`

## 制限事項

- 絵文字、縦書き、`textPath`は未対応
- 外部リソース（URL、CSS）は禁止
- グラデーション、パターン、フィルタは未対応
