package font

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
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

	// フォントフォールバックの決定
	actualFontFamily := fontFamily
	actualFontStyle := fontStyle

	// Arial Unicodeを優先的に使用
	if fontFamily == "Arial" || fontFamily == "Arial Unicode" {
		if _, exists := r.fonts["Arial Unicode-Regular"]; exists {
			actualFontFamily = "Arial Unicode"
			actualFontStyle = "Regular"
			log.Printf("Using Arial Unicode for Arial font family")
		}
	}

	// フォントが見つからない場合のフォールバック
	if _, exists := r.fonts[fmt.Sprintf("%s-%s", actualFontFamily, actualFontStyle)]; !exists {
		log.Printf("Font not found: %s-%s, using basicfont fallback", actualFontFamily, actualFontStyle)
		return r.shapeWithBasicFont(text, fontSize)
	}

	// 基本的なテキストラン情報を作成
	run := &TextRun{
		Text:       text,
		FontFamily: actualFontFamily,
		FontSize:   fontSize,
		FontStyle:  actualFontStyle,
		X:          0,
		Y:          0,
		Glyphs:     []*GlyphInfo{},
	}

	// フォントサイズに基づいて幅と高さを計算
	if actualFontFamily == "Arial Unicode" {
		// Arial Unicodeフォントの場合
		run.Width = float64(len(text)) * fontSize * 0.6 // 概算の文字幅
		run.Height = fontSize * 1.2                     // 概算の文字高さ
	} else {
		// basicfontの場合
		scale := fontSize / 13.0
		run.Width = float64(len(text)) * 7 * scale
		run.Height = 13 * scale
	}

	log.Printf("Text shaped successfully: %s characters, width=%f, height=%f", len(run.Glyphs), run.Width, run.Height)
	return run, nil
}

// shapeWithBasicFont はbasicfontを使用してテキストをシェイピングします
func (r *Renderer) shapeWithBasicFont(text string, fontSize float64) (*TextRun, error) {
	// basicfontのメトリクスを使用
	scale := fontSize / 13.0 // basicfontの高さは13px

	run := &TextRun{
		Text:       text,
		FontFamily: "BasicFont",
		FontSize:   fontSize,
		FontStyle:  "Regular",
		X:          0,
		Y:          0,
		Glyphs:     []*GlyphInfo{},
		Width:      float64(len(text)) * 7 * scale, // basicfontの文字幅は7px
		Height:     13 * scale,
	}

	// 各文字のグリフ情報を作成
	for i := range text {
		glyph := &GlyphInfo{
			X:        float64(i) * 7 * scale,
			Y:        0,
			Width:    int(7 * scale),
			Height:   int(13 * scale),
			Advance:  7 * scale,
			Baseline: 0,
		}
		run.Glyphs = append(run.Glyphs, glyph)
	}

	return run, nil
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

	// 画像サイズの計算（パディングを追加）
	padding := 2
	width := int(math.Ceil(float64(bounds.Max.X-bounds.Min.X)/64.0)) + padding*2
	height := int(math.Ceil(float64(bounds.Max.Y-bounds.Min.Y)/64.0)) + padding*2

	if width <= 0 || height <= 0 {
		width = 1
		height = 1
	}

	// 画像の作成
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// より正確なグリフ描画の実装
	// グリフの輪郭を取得して描画
	if err := r.drawGlyphOutline(fontFace, glyphIndex, img, padding); err != nil {
		// フォールバック: 簡易的な描画
		r.drawSimpleGlyph(img, width, height)
	}

	return img, nil
}

// drawGlyphOutline はグリフの輪郭を描画します
func (r *Renderer) drawGlyphOutline(fontFace *FontFace, glyphIndex sfnt.GlyphIndex, img *image.RGBA, padding int) error {
	// グリフの輪郭を取得
	segments, err := fontFace.Font.LoadGlyph(nil, glyphIndex, fixed.Int26_6(fontFace.Size*64), nil)
	if err != nil {
		return fmt.Errorf("failed to load glyph: %w", err)
	}

	// 輪郭を描画
	for _, segment := range segments {
		switch segment.Op {
		case sfnt.SegmentOpMoveTo:
			// 移動
			x := int(segment.Args[0].X/64) + padding
			y := int(segment.Args[0].Y/64) + padding
			if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
				img.Set(x, y, color.RGBA{0, 0, 0, 255})
			}
		case sfnt.SegmentOpLineTo:
			// 直線
			x := int(segment.Args[0].X/64) + padding
			y := int(segment.Args[0].Y/64) + padding
			if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
				img.Set(x, y, color.RGBA{0, 0, 0, 255})
			}
		case sfnt.SegmentOpQuadTo:
			// 二次ベジェ曲線（簡易的な実装）
			x := int(segment.Args[1].X/64) + padding
			y := int(segment.Args[1].Y/64) + padding
			if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
				img.Set(x, y, color.RGBA{0, 0, 0, 255})
			}
		case sfnt.SegmentOpCubeTo:
			// 三次ベジェ曲線（簡易的な実装）
			x := int(segment.Args[2].X/64) + padding
			y := int(segment.Args[2].Y/64) + padding
			if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
				img.Set(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
	}

	// 輪郭を塗りつぶし
	r.fillGlyphOutline(img)
	return nil
}

// fillGlyphOutline はグリフの輪郭を塗りつぶします
func (r *Renderer) fillGlyphOutline(img *image.RGBA) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	// フラッドフィルアルゴリズムで塗りつぶし
	for y := 0; y < height; y++ {
		fill := false
		for x := 0; x < width; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a > 0 {
				fill = !fill
			}
			if fill {
				img.Set(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
	}
}

// drawSimpleGlyph は簡易的なグリフ描画（フォールバック）
func (r *Renderer) drawSimpleGlyph(img *image.RGBA, width, height int) {
	// basicfontを使用したより良いフォールバック
	// 基本的な文字形状を描画
	centerX := width / 2
	centerY := height / 2

	// 文字の基本形状を描画（例: 長方形の文字）
	charWidth := width * 3 / 4
	charHeight := height * 3 / 4
	startX := centerX - charWidth/2
	startY := centerY - charHeight/2

	// 文字の輪郭を描画
	for y := startY; y < startY+charHeight; y++ {
		for x := startX; x < startX+charWidth; x++ {
			if x >= 0 && x < width && y >= 0 && y < height {
				// 輪郭のみ描画
				if x == startX || x == startX+charWidth-1 || y == startY || y == startY+charHeight-1 {
					img.Set(x, y, color.RGBA{0, 0, 0, 255})
				}
			}
		}
	}
}

// drawWithBasicFont はbasicfontを使用してテキストを描画します
func (r *Renderer) drawWithBasicFont(text string, target *image.RGBA, x, y float64, textColor color.Color) error {
	log.Printf("drawWithBasicFont called with text: '%s' at (%f, %f) with color: %v", text, x, y, textColor)

	// basicfontは固定サイズなので、スケーラブルなフォントを優先的に使用
	// Genevaフォントが利用可能な場合は使用
	if fontFace, exists := r.fonts["Geneva-Regular"]; exists {
		return r.drawWithScalableFont(text, target, x, y, 12.0, textColor, fontFace) // デフォルトサイズ
	}

	// 最もシンプルで確実な描画方法（basicfont）
	d := &font.Drawer{
		Dst:  target,
		Src:  image.NewUniform(textColor),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6((y + 13) * 64)}, // ベースライン調整
	}
	d.DrawString(text)

	log.Printf("Text drawing completed with basicfont (simple)")
	return nil
}

// drawWithScalableFont はスケーラブルなフォントでテキストを描画します
func (r *Renderer) drawWithScalableFont(text string, target *image.RGBA, x, y float64, fontSize float64, textColor color.Color, fontFace *FontFace) error {
	log.Printf("drawWithScalableFont called with text: '%s' at (%f, %f) with size %f and color: %v", text, x, y, fontSize, textColor)

	// スケーラブルなフォントで描画
	face, err := opentype.NewFace(fontFace.OTFont, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     96,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Printf("Failed to create font face, falling back to basicfont: %v", err)
		return r.drawWithBasicFont(text, target, x, y, textColor)
	}
	defer face.Close()

	// ベースライン位置を調整（SVGのy座標はベースライン位置）
	baselineY := y + fontSize*0.7 // フォントサイズの70%をベースラインオフセットとして使用

	d := &font.Drawer{
		Dst:  target,
		Src:  image.NewUniform(textColor),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(baselineY * 64)},
	}
	d.DrawString(text)

	log.Printf("Text drawing completed with scalable font (size: %f)", fontSize)
	return nil
}

// RenderTextRun はテキストランを描画します
func (r *Renderer) RenderTextRun(run *TextRun, target *image.RGBA, x, y float64, textColor color.Color) error {
	log.Printf("Rendering text run: %s at (%f, %f) with color: %v, font size: %f", run.Text, x, y, textColor, run.FontSize)

	// Arial Unicodeフォントが利用可能な場合は使用
	if run.FontFamily == "Arial Unicode" {
		return r.drawWithArialUnicode(run.Text, target, x, y, run.FontSize, textColor)
	}

	// Genevaフォントが利用可能な場合は使用
	if fontFace, exists := r.fonts["Geneva-Regular"]; exists {
		return r.drawWithScalableFont(run.Text, target, x, y, run.FontSize, textColor, fontFace)
	}

	// その他の場合はbasicfontを使用（サイズは無視されるが、スケールされたサイズを記録）
	log.Printf("Using basicfont with scaled size: %f (will be ignored)", run.FontSize)
	return r.drawWithBasicFont(run.Text, target, x, y, textColor)
}

// drawWithArialUnicode はArial Unicodeフォントでテキストを描画します
func (r *Renderer) drawWithArialUnicode(text string, target *image.RGBA, x, y float64, fontSize float64, textColor color.Color) error {
	log.Printf("drawWithArialUnicode called with text: '%s' at (%f, %f) with size %f and color: %v", text, x, y, fontSize, textColor)

	// Arial Unicodeフォントを取得
	fontFace, exists := r.fonts["Arial Unicode-Regular"]
	if !exists {
		log.Printf("Arial Unicode font not found, falling back to basicfont")
		return r.drawWithBasicFont(text, target, x, y, textColor)
	}

	// シンプルで確実な描画方法
	face, err := opentype.NewFace(fontFace.OTFont, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     96,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Printf("Failed to create font face, falling back to basicfont: %v", err)
		return r.drawWithBasicFont(text, target, x, y, textColor)
	}
	defer face.Close()

	// ベースライン位置を調整（SVGのy座標はベースライン位置）
	baselineY := y + fontSize*0.7 // フォントサイズの70%をベースラインオフセットとして使用

	d := &font.Drawer{
		Dst:  target,
		Src:  image.NewUniform(textColor),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(baselineY * 64)},
	}
	d.DrawString(text)

	log.Printf("Text drawing completed with Arial Unicode (simple)")
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
