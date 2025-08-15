package svg2png

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderPNG_Basic(t *testing.T) {
	// テスト用の簡単なSVG
	svgData := []byte(`<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
		<rect x="10" y="10" width="80" height="80" fill="red"/>
	</svg>`)

	opts := Options{
		Width:  200,
		Height: 200,
		DPI:    96,
	}

	pngData, diag, err := RenderPNG(svgData, opts)
	if err != nil {
		t.Fatalf("RenderPNG failed: %v", err)
	}

	if len(pngData) == 0 {
		t.Error("PNG data is empty")
	}

	// 診断情報の確認
	if len(diag.Warnings) > 0 {
		t.Logf("Warnings: %v", diag.Warnings)
	}

	// PNGデータが有効かチェック（PNGヘッダー）
	if !bytes.HasPrefix(pngData, []byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Error("Invalid PNG header")
	}
}

func TestRenderPNG_Text(t *testing.T) {
	// テキストを含むSVG
	svgData := []byte(`<svg width="200" height="100" xmlns="http://www.w3.org/2000/svg">
		<text x="100" y="50" font-family="Arial" font-size="20" text-anchor="middle" fill="black">Hello World</text>
	</svg>`)

	opts := Options{
		Width:  400,
		Height: 200,
		DPI:    96,
	}

	pngData, diag, err := RenderPNG(svgData, opts)
	if err != nil {
		t.Fatalf("RenderPNG with text failed: %v", err)
	}

	if len(pngData) == 0 {
		t.Error("PNG data is empty")
	}

	// フォント関連の警告があればログ出力
	if len(diag.MissingFonts) > 0 {
		t.Logf("Missing fonts: %v", diag.MissingFonts)
	}
}

func TestRenderPNG_WithBackground(t *testing.T) {
	svgData := []byte(`<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
		<circle cx="50" cy="50" r="40" fill="blue"/>
	</svg>`)

	// 白い背景を指定
	white := color.RGBA{255, 255, 255, 255}
	opts := Options{
		Width:      200,
		Height:     200,
		DPI:        96,
		Background: &white,
	}

	pngData, _, err := RenderPNG(svgData, opts)
	if err != nil {
		t.Fatalf("RenderPNG with background failed: %v", err)
	}

	if len(pngData) == 0 {
		t.Error("PNG data is empty")
	}
}

func TestRenderPNG_TransparentBackground(t *testing.T) {
	svgData := []byte(`<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
		<rect x="20" y="20" width="60" height="60" fill="green" opacity="0.8"/>
	</svg>`)

	// 透過背景（Background: nil）
	opts := Options{
		Width:  200,
		Height: 200,
		DPI:    96,
	}

	pngData, _, err := RenderPNG(svgData, opts)
	if err != nil {
		t.Fatalf("RenderPNG with transparent background failed: %v", err)
	}

	if len(pngData) == 0 {
		t.Error("PNG data is empty")
	}
}

func TestRenderPNG_FromFile(t *testing.T) {
	// examplesディレクトリのsimple.svgを読み込み
	svgPath := filepath.Join("..", "..", "examples", "simple.svg")
	svgData, err := os.ReadFile(svgPath)
	if err != nil {
		t.Skipf("Could not read test SVG file: %v", err)
	}

	opts := Options{
		Width:  800,
		Height: 600,
		DPI:    96,
	}

	pngData, _, err := RenderPNG(svgData, opts)
	if err != nil {
		t.Fatalf("RenderPNG from file failed: %v", err)
	}

	if len(pngData) == 0 {
		t.Error("PNG data is empty")
	}
}

func TestRenderPNG_TextFromFile(t *testing.T) {
	// examplesディレクトリのtext_test.svgを読み込み
	svgPath := filepath.Join("..", "..", "examples", "text_test.svg")
	svgData, err := os.ReadFile(svgPath)
	if err != nil {
		t.Skipf("Could not read test SVG file: %v", err)
	}

	opts := Options{
		Width:  800,
		Height: 600,
		DPI:    96,
	}

	pngData, _, err := RenderPNG(svgData, opts)
	if err != nil {
		t.Fatalf("RenderPNG text test from file failed: %v", err)
	}

	if len(pngData) == 0 {
		t.Error("PNG data is empty")
	}
}

func TestRenderPNG_InvalidSVG(t *testing.T) {
	// 無効なSVGデータ（構文エラー）
	invalidSVG := []byte(`<svg><rect x="10" y="10" width="80" height="80" fill="red"`)

	opts := Options{
		Width:  100,
		Height: 100,
		DPI:    96,
	}

	_, _, err := RenderPNG(invalidSVG, opts)
	if err == nil {
		t.Error("Expected error for invalid SVG, but got none")
	}
}

func TestRenderPNG_EmptySVG(t *testing.T) {
	// 空のSVG
	emptySVG := []byte(`<svg></svg>`)

	opts := Options{
		Width:  100,
		Height: 100,
		DPI:    96,
	}

	pngData, _, err := RenderPNG(emptySVG, opts)
	if err != nil {
		t.Fatalf("RenderPNG with empty SVG failed: %v", err)
	}

	if len(pngData) == 0 {
		t.Error("PNG data is empty")
	}
}

func TestRenderPNG_Options(t *testing.T) {
	svgData := []byte(`<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
		<rect x="10" y="10" width="80" height="80" fill="purple"/>
	</svg>`)

	tests := []struct {
		name     string
		opts     Options
		expected bool
	}{
		{
			name: "Default DPI",
			opts: Options{Width: 100, Height: 100},
		},
		{
			name: "Custom DPI",
			opts: Options{Width: 100, Height: 100, DPI: 300},
		},
		{
			name: "Custom font family",
			opts: Options{Width: 100, Height: 100, DefaultFamily: "Helvetica"},
		},
		{
			name: "System font scan enabled",
			opts: Options{Width: 100, Height: 100, SystemFontScan: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pngData, _, err := RenderPNG(svgData, tt.opts)
			if err != nil {
				t.Errorf("RenderPNG failed: %v", err)
				return
			}

			if len(pngData) == 0 {
				t.Error("PNG data is empty")
			}
		})
	}
}

func TestRenderPNG_ComplexShapes(t *testing.T) {
	// 複雑な図形を含むSVG
	svgData := []byte(`<svg width="200" height="200" xmlns="http://www.w3.org/2000/svg">
		<rect x="10" y="10" width="80" height="80" fill="red" stroke="black" stroke-width="2"/>
		<circle cx="150" cy="50" r="30" fill="blue" stroke="darkblue" stroke-width="3"/>
		<path d="M 50 150 L 100 120 L 150 150 Z" fill="green" stroke="darkgreen" stroke-width="2"/>
		<text x="100" y="180" font-family="Arial" font-size="16" text-anchor="middle" fill="black">Complex SVG</text>
	</svg>`)

	opts := Options{
		Width:  400,
		Height: 400,
		DPI:    96,
	}

	pngData, _, err := RenderPNG(svgData, opts)
	if err != nil {
		t.Fatalf("RenderPNG with complex shapes failed: %v", err)
	}

	if len(pngData) == 0 {
		t.Error("PNG data is empty")
	}
}
