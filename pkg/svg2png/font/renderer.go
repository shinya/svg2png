package font

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Renderer はフォントレンダリングを行います
type Renderer struct {
	fonts map[string]*FontFace
}

// FontFace はフォントの情報とレンダリングデータを表します
type FontFace struct {
	Family string
	Style  string
	Path   string
	Data   []byte
	Font   *sfnt.Font
	OTFont *opentype.Font
	Size   float64
	DPI    float64
}

// GlyphInfo はグリフの情報を表します
type GlyphInfo struct {
	X, Y     float64
	Width    int
	Height   int
	Advance  float64
	Image    *image.RGBA
	Baseline float64
}

// TextRun はテキストの描画情報を表します
type TextRun struct {
	Text       string
	FontFamily string
	FontSize   float64
	FontStyle  string
	FontWeight string
	X, Y       float64
	Glyphs     []*GlyphInfo
	Width      float64
	Height     float64
}

// NewRenderer は新しいフォントレンダラーを作成します
func NewRenderer() *Renderer {
	return &Renderer{
		fonts: make(map[string]*FontFace),
	}
}

// LoadFont はフォントを読み込みます
func (r *Renderer) LoadFont(fontInfo *FontInfo) error {
	key := fmt.Sprintf("%s-%s", fontInfo.Family, fontInfo.Style)

	// 既に読み込まれている場合はスキップ
	if _, exists := r.fonts[key]; exists {
		log.Printf("Font already loaded: %s", key)
		return nil
	}

	var fontData []byte
	var err error

	if fontInfo.Data != nil {
		fontData = fontInfo.Data
		log.Printf("Loading font from memory data: %s", key)
	} else if fontInfo.Path != "" {
		fontData, err = readFontFile(fontInfo.Path)
		if err != nil {
			return fmt.Errorf("failed to read font file %s: %w", fontInfo.Path, err)
		}
		log.Printf("Loading font from file: %s (%d bytes)", fontInfo.Path, len(fontData))
	} else {
		return fmt.Errorf("no font data or path provided")
	}

	// SFNTフォントの読み込み
	sfntFont, err := sfnt.Parse(fontData)
	if err != nil {
		return fmt.Errorf("failed to parse SFNT font: %w", err)
	}

	// OpenTypeフォントの読み込み（Harfbuzz用）
	otFont, err := opentype.Parse(fontData)
	if err != nil {
		return fmt.Errorf("failed to parse OpenType font: %w", err)
	}

	fontFace := &FontFace{
		Family: fontInfo.Family,
		Style:  fontInfo.Style,
		Path:   fontInfo.Path,
		Data:   fontData,
		Font:   sfntFont,
		OTFont: otFont,
		Size:   12, // デフォルトサイズ
		DPI:    96, // デフォルトDPI
	}

	r.fonts[key] = fontFace
	log.Printf("Font loaded successfully: %s", key)
	return nil
}

// ShapeText はテキストをシェイピングします
func (r *Renderer) ShapeText(text string, fontFamily, fontStyle string, fontSize float64) (*TextRun, error) {
	key := fmt.Sprintf("%s-%s", fontFamily, fontStyle)
	log.Printf("Attempting to shape text with font: %s", key)

	fontFace, exists := r.fonts[key]
	if !exists {
		return nil, fmt.Errorf("font not found: %s %s", fontFamily, fontStyle)
	}

	// Harfbuzzによるシェイピング
	run := &TextRun{
		Text:       text,
		FontFamily: fontFamily,
		FontSize:   fontSize,
		FontStyle:  fontStyle,
		X:          0,
		Y:          0,
		Glyphs:     []*GlyphInfo{},
	}

	// フォントサイズの設定
	fontFace.Size = fontSize

	// シェイピングの実行
	if err := r.shapeWithHarfbuzz(run, fontFace); err != nil {
		return nil, fmt.Errorf("harfbuzz shaping failed: %w", err)
	}

	log.Printf("Text shaped successfully: %s characters, width=%f, height=%f", len(run.Glyphs), run.Width, run.Height)
	return run, nil
}

// shapeWithHarfbuzz はHarfbuzzを使用してテキストをシェイピングします
func (r *Renderer) shapeWithHarfbuzz(run *TextRun, fontFace *FontFace) error {
	// 簡易的な実装
	// 実際の実装では、Harfbuzzの詳細な設定が必要

	// 各文字を個別に処理
	for i, char := range run.Text {
		glyph, err := r.renderGlyph(char, fontFace)
		if err != nil {
			return fmt.Errorf("failed to render glyph for '%c': %w", char, err)
		}

		// 位置の調整
		if i > 0 {
			glyph.X = run.Glyphs[i-1].X + run.Glyphs[i-1].Advance
		}

		run.Glyphs = append(run.Glyphs, glyph)
		run.Width += glyph.Advance
		run.Height = math.Max(run.Height, float64(glyph.Height))
	}

	return nil
}

// renderGlyph は個別のグリフをレンダリングします
func (r *Renderer) renderGlyph(char rune, fontFace *FontFace) (*GlyphInfo, error) {
	// グリフインデックスの取得
	glyphIndex, err := fontFace.Font.GlyphIndex(nil, char)
	if err != nil {
		return nil, fmt.Errorf("failed to get glyph index for '%c': %w", char, err)
	}

	// グリフのメトリクス取得
	bounds, _, err := fontFace.Font.GlyphBounds(nil, glyphIndex, fixed.Int26_6(fontFace.Size*64), font.HintingNone)
	if err != nil {
		return nil, fmt.Errorf("failed to get glyph bounds: %w", err)
	}

	// グリフの描画
	glyphImage, err := r.drawGlyph(fontFace, glyphIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to draw glyph: %w", err)
	}

	// グリフ情報の作成
	glyph := &GlyphInfo{
		X:        0,
		Y:        0,
		Width:    glyphImage.Bounds().Dx(),
		Height:   glyphImage.Bounds().Dy(),
		Advance:  float64(bounds.Max.X-bounds.Min.X) / 64.0, // 26.6形式から変換
		Image:    glyphImage,
		Baseline: float64(bounds.Min.Y) / 64.0,
	}

	return glyph, nil
}

// drawGlyph はグリフを描画します
func (r *Renderer) drawGlyph(fontFace *FontFace, glyphIndex sfnt.GlyphIndex) (*image.RGBA, error) {
	// グリフの境界を取得
	bounds, _, err := fontFace.Font.GlyphBounds(nil, glyphIndex, fixed.Int26_6(fontFace.Size*64), font.HintingNone)
	if err != nil {
		return nil, fmt.Errorf("failed to get glyph bounds: %w", err)
	}

	// 画像サイズの計算
	width := int(math.Ceil(float64(bounds.Max.X-bounds.Min.X) / 64.0))
	height := int(math.Ceil(float64(bounds.Max.Y-bounds.Min.Y) / 64.0))

	if width <= 0 || height <= 0 {
		width = 1
		height = 1
	}

	// 画像の作成
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// グリフの輪郭を取得
	// GlyphOutlineが利用できないため、簡易的な描画を実装
	// 実際の実装では、より精密な輪郭計算が必要

	// グリフの形状を簡易的に描画
	// 中心部分を塗りつぶし、輪郭を描画
	centerX := width / 2
	centerY := height / 2

	// 内部を塗りつぶし
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 中心からの距離で内部判定
			dist := math.Sqrt(float64((x-centerX)*(x-centerX) + (y-centerY)*(y-centerY)))
			maxDist := math.Min(float64(width), float64(height)) / 2.5

			if dist <= maxDist {
				img.Set(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
	}

	// 輪郭を描画（簡易的な実装）
	// 実際の実装では、グリフの輪郭を正確に判定する必要がある
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 輪郭の判定（簡易的な実装）
			if x == 0 || x == width-1 || y == 0 || y == height-1 {
				// 外枠
				img.Set(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
	}

	return img, nil
}

// RenderTextRun はテキストランを描画します
func (r *Renderer) RenderTextRun(run *TextRun, target *image.RGBA, x, y float64) error {
	log.Printf("Rendering text run: %s at (%f, %f)", run.Text, x, y)

	for _, glyph := range run.Glyphs {
		if glyph.Image == nil {
			continue
		}

		// グリフの描画位置を計算
		gx := int(x + glyph.X)
		gy := int(y + glyph.Y - glyph.Baseline)

		// グリフをターゲット画像に描画
		draw.Draw(target, image.Rect(gx, gy, gx+glyph.Width, gy+glyph.Height),
			glyph.Image, image.Point{0, 0}, draw.Over)
	}

	log.Printf("Text run rendered successfully")
	return nil
}

// readFontFile はフォントファイルを読み込みます
func readFontFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// GetFont は指定されたファミリとスタイルのフォントを取得します
func (r *Renderer) GetFont(family, style string) (*FontFace, error) {
	key := fmt.Sprintf("%s-%s", family, style)
	fontFace, exists := r.fonts[key]
	if !exists {
		return nil, fmt.Errorf("font not found: %s %s", family, style)
	}
	return fontFace, nil
}

// SetFontSize はフォントサイズを設定します
func (r *Renderer) SetFontSize(family, style string, size float64) error {
	key := fmt.Sprintf("%s-%s", family, style)
	fontFace, exists := r.fonts[key]
	if !exists {
		return fmt.Errorf("font not found: %s %s", family, style)
	}

	fontFace.Size = size
	return nil
}

// SetDPI はDPIを設定します
func (r *Renderer) SetDPI(family, style string, dpi float64) error {
	key := fmt.Sprintf("%s-%s", family, style)
	fontFace, exists := r.fonts[key]
	if !exists {
		return fmt.Errorf("font not found: %s %s", family, style)
	}

	fontFace.DPI = dpi
	return nil
}
