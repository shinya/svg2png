package font

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"strings"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Renderer はフォントレンダリングを行います
type Renderer struct {
	fonts map[string]*FontFace // "Family-Style" → FontFace
}

// FontFace はフォントのメタデータとデータを保持します
type FontFace struct {
	Family string
	Style  string
	Path   string
	Data   []byte
	Font   *sfnt.Font
	OTFont *opentype.Font
}

// GlyphInfo はグリフ情報（互換性のために残す）
type GlyphInfo struct {
	X, Y     float64
	Width    int
	Height   int
	Advance  float64
	Image    *image.RGBA
	Baseline float64
}

// TextRun はテキスト描画情報（互換性のために残す）
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

	if _, exists := r.fonts[key]; exists {
		return nil
	}

	var fontData []byte
	var err error

	if len(fontInfo.Data) > 0 {
		fontData = fontInfo.Data
	} else if fontInfo.Path != "" {
		fontData, err = os.ReadFile(fontInfo.Path)
		if err != nil {
			return fmt.Errorf("failed to read font file %s: %w", fontInfo.Path, err)
		}
	} else {
		return fmt.Errorf("no font data or path provided")
	}

	sfntFont, err := sfnt.Parse(fontData)
	if err != nil {
		return fmt.Errorf("failed to parse SFNT: %w", err)
	}

	otFont, err := opentype.Parse(fontData)
	if err != nil {
		return fmt.Errorf("failed to parse OpenType: %w", err)
	}

	r.fonts[key] = &FontFace{
		Family: fontInfo.Family,
		Style:  fontInfo.Style,
		Path:   fontInfo.Path,
		Data:   fontData,
		Font:   sfntFont,
		OTFont: otFont,
	}

	log.Printf("Font loaded: %s", key)
	return nil
}

// GetFont は指定されたキーのフォントを取得します
func (r *Renderer) GetFont(family, style string) (*FontFace, error) {
	key := fmt.Sprintf("%s-%s", family, style)
	ff, ok := r.fonts[key]
	if !ok {
		return nil, fmt.Errorf("font not found: %s", key)
	}
	return ff, nil
}

// RenderText はテキストをターゲット画像に描画します
// x, y はSVGのテキストベースライン位置（ピクセル座標）
func (r *Renderer) RenderText(text, family, style string, fontSize float64, target *image.RGBA, x, y float64, col color.Color) error {
	ff := r.FindFont(family, style)
	if ff != nil {
		return r.renderWithOpenType(text, ff, fontSize, target, x, y, col)
	}
	// 最終フォールバック: basicfont
	return r.renderWithBasicFont(text, fontSize, target, x, y, col)
}

// renderWithOpenType はOpenTypeフォントでテキストを描画します
func (r *Renderer) renderWithOpenType(text string, ff *FontFace, fontSize float64, target *image.RGBA, x, y float64, col color.Color) error {
	face, err := opentype.NewFace(ff.OTFont, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     96,
		Hinting: xfont.HintingFull,
	})
	if err != nil {
		return fmt.Errorf("failed to create font face: %w", err)
	}
	defer face.Close()

	// SVGのy属性はベースライン位置を示す
	// font.Drawer の Dot.Y もベースライン位置なので、そのまま使用する
	d := &xfont.Drawer{
		Dst:  target,
		Src:  image.NewUniform(col),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(x * 64),
			Y: fixed.Int26_6(y * 64),
		},
	}
	d.DrawString(text)
	return nil
}

// renderWithBasicFont はbasicfontでテキストを描画します（最終フォールバック）
func (r *Renderer) renderWithBasicFont(text string, fontSize float64, target *image.RGBA, x, y float64, col color.Color) error {
	// basicfontのスケール（basicfontは13pxで設計されている）
	// basicfontは固定サイズのため、サイズ変換はできないが、
	// 位置の調整（basicfontの ascent = 11px）は必要
	const basicAscent = 11.0 // basicfont.Face7x13 のアセント

	d := &xfont.Drawer{
		Dst:  target,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(x * 64),
			Y: fixed.Int26_6((y - basicAscent + 13) * 64), // basicfontのベースライン補正
		},
	}
	d.DrawString(text)

	log.Printf("Text rendered with basicfont (fallback)")
	return nil
}

// FindFont はファミリ名とスタイルからフォントを探します
// 見つからない場合は nil を返します（basicfont フォールバックを示す）
func (r *Renderer) FindFont(family, style string) *FontFace {
	key := fmt.Sprintf("%s-%s", family, style)
	if ff, exists := r.fonts[key]; exists {
		return ff
	}
	keyReg := fmt.Sprintf("%s-Regular", family)
	if ff, exists := r.fonts[keyReg]; exists {
		return ff
	}
	// 部分一致（前方一致、大文字小文字区別なし）
	lowerFamily := strings.ToLower(family)
	for k, ff := range r.fonts {
		if strings.HasPrefix(strings.ToLower(k), lowerFamily+"-") {
			return ff
		}
	}
	return nil
}

// MeasureText はテキストの描画幅を計算します
// フォントが見つからない場合は error を返します（無音フォールバックなし）
func (r *Renderer) MeasureText(text, family, style string, fontSize float64) (float64, error) {
	ff := r.FindFont(family, style)
	if ff == nil {
		return 0, fmt.Errorf("font not found: %s %s", family, style)
	}
	return r.measureWithOpenType(text, ff, fontSize)
}

// measureWithOpenType はOpenTypeフォントでテキスト幅を計算します
func (r *Renderer) measureWithOpenType(text string, ff *FontFace, fontSize float64) (float64, error) {
	face, err := opentype.NewFace(ff.OTFont, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     96,
		Hinting: xfont.HintingFull,
	})
	if err != nil {
		return 0, err
	}
	defer face.Close()

	d := &xfont.Drawer{Face: face}
	advance := d.MeasureString(text)
	return float64(advance) / 64.0, nil
}

// ShapeText は互換性のために残す（レガシーAPI）
func (r *Renderer) ShapeText(text, fontFamily, fontStyle string, fontSize float64) (*TextRun, error) {
	key := fmt.Sprintf("%s-%s", fontFamily, fontStyle)

	width := float64(len([]rune(text))) * fontSize * 0.6
	height := fontSize * 1.2

	if ff, exists := r.fonts[key]; exists {
		if w, err := r.measureWithOpenType(text, ff, fontSize); err == nil {
			width = w
		}
	}

	return &TextRun{
		Text:       text,
		FontFamily: fontFamily,
		FontSize:   fontSize,
		FontStyle:  fontStyle,
		Width:      width,
		Height:     height,
	}, nil
}

// RenderTextRun は互換性のために残す（レガシーAPI）
func (r *Renderer) RenderTextRun(run *TextRun, target *image.RGBA, x, y float64, textColor color.Color) error {
	return r.RenderText(run.Text, run.FontFamily, run.FontStyle, run.FontSize, target, x, y, textColor)
}
