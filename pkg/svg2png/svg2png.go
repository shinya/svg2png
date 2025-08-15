package svg2png

import (
	"fmt"
	"image/color"

	"github.com/shinya/svg2png/pkg/svg2png/font"
	"github.com/shinya/svg2png/pkg/svg2png/parser"
	"github.com/shinya/svg2png/pkg/svg2png/raster"
	"github.com/shinya/svg2png/pkg/svg2png/renderer"
	"github.com/shinya/svg2png/pkg/svg2png/style"
	"github.com/shinya/svg2png/pkg/svg2png/viewport"
)

// FontSource はフォントの供給源を表します
type FontSource = font.FontSource

// Options はレンダリングオプションを表します
type Options struct {
	Width, Height  int
	DPI            float64     // 既定 96
	Background     *color.RGBA // nilで透過
	DefaultFamily  string      // 既定フォント（fallback最終手段）
	SystemFontScan bool        // trueで起動時スキャン
}

// Diagnostics は診断情報を表します
type Diagnostics struct {
	Warnings     []string
	MissingFonts []string
	Unsupported  []string // 未対応属性名など
}

// グローバルフォントマネージャー
var globalFontManager *font.Manager

func init() {
	globalFontManager = font.NewManager()
}

// RegisterFonts はフォントを登録します
func RegisterFonts(fonts ...FontSource) error {
	return globalFontManager.RegisterFonts(fonts...)
}

// ClearFontCache はフォントキャッシュをクリアします
func ClearFontCache() {
	globalFontManager.ClearCache()
}

// RenderPNG はSVGをPNGに変換します
func RenderPNG(svg []byte, opts Options) (png []byte, diag Diagnostics, err error) {
	// デフォルト値の設定
	if opts.DPI == 0 {
		opts.DPI = 96
	}
	if opts.DefaultFamily == "" {
		opts.DefaultFamily = "Arial"
	}

	// システムフォントスキャンが有効な場合
	if opts.SystemFontScan {
		if err := globalFontManager.ScanSystemFonts(); err != nil {
			// 警告として記録するが、処理は続行
			diag.Warnings = append(diag.Warnings, fmt.Sprintf("System font scan failed: %v", err))
		}
	}

	// SVGパース
	doc, err := parser.ParseSVG(svg)
	if err != nil {
		return nil, Diagnostics{}, err
	}

	// ビューポート解決
	vp, err := viewport.ResolveViewport(doc, opts.Width, opts.Height, opts.DPI)
	if err != nil {
		return nil, Diagnostics{}, err
	}

	// スタイル解決器作成
	styleResolver := style.NewResolver(opts.DefaultFamily)

	// フレームバッファ作成
	fb := raster.NewFrameBuffer(opts.Width, opts.Height, opts.Background)

	// フォントレンダラーを取得
	fontRenderer := globalFontManager.GetRenderer()

	// レンダリングコンテキスト作成
	rc := raster.NewRasterContext(fb, fontRenderer, vp)

	// 要素の描画
	err = renderer.RenderElements(doc, vp, styleResolver, rc)
	if err != nil {
		return nil, Diagnostics{}, err
	}

	// PNGエンコード
	pngData, err := fb.EncodePNG()
	if err != nil {
		return nil, Diagnostics{}, err
	}

	// 診断情報収集
	styleDiag := styleResolver.GetDiagnostics()
	diag = Diagnostics{
		Warnings:     styleDiag.Warnings,
		MissingFonts: styleDiag.MissingFonts,
		Unsupported:  styleDiag.Unsupported,
	}

	return pngData, diag, nil
}
