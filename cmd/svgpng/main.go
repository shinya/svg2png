package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"
	"strings"

	"github.com/shinya/svg2png/pkg/svg2png"
)

func main() {
	// コマンドラインオプションの定義
	var (
		inputFile      = flag.String("in", "", "入力SVGファイル")
		outputFile     = flag.String("out", "", "出力PNGファイル")
		width          = flag.Int("w", 800, "出力幅")
		height         = flag.Int("h", 600, "出力高さ")
		background     = flag.String("bg", "transparent", "背景色（transparent、#RRGGBB、色名）")
		dpi            = flag.Float64("dpi", 96, "DPI")
		systemFontScan = flag.Bool("system-font-scan", false, "システムフォントのスキャンを有効にする")
		help           = flag.Bool("help", false, "ヘルプを表示")
	)

	flag.Parse()

	if *help {
		printUsage()
		return
	}

	// 必須オプションのチェック
	if *inputFile == "" || *outputFile == "" {
		log.Fatal("入力ファイル（-in）と出力ファイル（-out）を指定してください")
	}

	// 入力ファイルの読み込み
	svgData, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("入力ファイルの読み込みに失敗: %v", err)
	}

	// 背景色の解析
	var bgColor *color.RGBA
	if *background != "transparent" {
		if c, err := parseColor(*background); err == nil {
			bgColor = &c
		} else {
			log.Printf("警告: 背景色の解析に失敗（%s）、透過を使用: %v", *background, err)
		}
	}

	// レンダリングオプションの設定
	opts := svg2png.Options{
		Width:          *width,
		Height:         *height,
		DPI:            *dpi,
		Background:     bgColor,
		SystemFontScan: *systemFontScan,
	}

	// SVGからPNGへの変換
	pngData, diag, err := svg2png.RenderPNG(svgData, opts)
	if err != nil {
		log.Fatalf("レンダリングに失敗: %v", err)
	}

	// 出力ファイルへの書き込み
	if err := os.WriteFile(*outputFile, pngData, 0644); err != nil {
		log.Fatalf("出力ファイルの書き込みに失敗: %v", err)
	}

	// 診断情報の表示
	if len(diag.Warnings) > 0 {
		fmt.Println("警告:")
		for _, warning := range diag.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}

	if len(diag.MissingFonts) > 0 {
		fmt.Println("不足フォント:")
		for _, font := range diag.MissingFonts {
			fmt.Printf("  - %s\n", font)
		}
	}

	if len(diag.Unsupported) > 0 {
		fmt.Println("未対応機能:")
		for _, feature := range diag.Unsupported {
			fmt.Printf("  - %s\n", feature)
		}
	}

	fmt.Printf("変換完了: %s -> %s (%dx%d)\n", *inputFile, *outputFile, *width, *height)
}

// printUsage は使用方法を表示します
func printUsage() {
	fmt.Print(`SVG2PNG - SVGからPNGへの変換ツール

使用方法:
  svgpng -in <入力ファイル> -out <出力ファイル> [オプション]

オプション:
  -in string
        入力SVGファイル（必須）
  -out string
        出力PNGファイル（必須）
  -w int
        出力幅（デフォルト: 800）
  -h int
        出力高さ（デフォルト: 600）
  -bg string
        背景色（デフォルト: transparent）
        例: transparent, #ffffff, white, black
  -dpi float
        DPI（デフォルト: 96）
  -system-font-scan
        システムフォントのスキャンを有効にする
  -help
        このヘルプを表示

例:
  svgpng -in input.svg -out output.png -w 1024 -h 768
  svgpng -in input.svg -out output.png -w 800 -h 600 -bg white
  svgpng -in input.svg -out output.png -w 1920 -h 1080 -bg #000000
  svgpng -in input.svg -out output.png -w 800 -h 600 -system-font-scan
`)
}

// parseColor は色文字列を解析します
func parseColor(colorStr string) (color.RGBA, error) {
	colorStr = strings.TrimSpace(colorStr)

	// 名前付き色
	if c, ok := namedColors[colorStr]; ok {
		return c, nil
	}

	// 16進数色
	if strings.HasPrefix(colorStr, "#") {
		return parseHexColor(colorStr)
	}

	return color.RGBA{}, fmt.Errorf("unsupported color format: %s", colorStr)
}

// parseHexColor は16進数色を解析します
func parseHexColor(hex string) (color.RGBA, error) {
	hex = strings.TrimPrefix(hex, "#")

	if len(hex) != 6 {
		return color.RGBA{}, fmt.Errorf("invalid hex color length: %s", hex)
	}

	var r, g, b uint8

	if rv, err := parseHexByte(hex[0:2]); err == nil {
		r = rv
	}
	if gv, err := parseHexByte(hex[2:4]); err == nil {
		g = gv
	}
	if bv, err := parseHexByte(hex[4:6]); err == nil {
		b = bv
	}

	return color.RGBA{r, g, b, 255}, nil
}

// parseHexByte は16進数のバイト値を解析します
func parseHexByte(hex string) (uint8, error) {
	if len(hex) != 2 {
		return 0, fmt.Errorf("invalid hex byte: %s", hex)
	}

	var result uint8
	_, err := fmt.Sscanf(hex, "%x", &result)
	return result, err
}

// namedColors は名前付き色のマップです
var namedColors = map[string]color.RGBA{
	"black":   {0, 0, 0, 255},
	"white":   {255, 255, 255, 255},
	"red":     {255, 0, 0, 255},
	"green":   {0, 255, 0, 255},
	"blue":    {0, 0, 255, 255},
	"yellow":  {255, 255, 0, 255},
	"cyan":    {0, 255, 255, 255},
	"magenta": {255, 0, 255, 255},
	"gray":    {128, 128, 128, 255},
	"grey":    {128, 128, 128, 255},
}
