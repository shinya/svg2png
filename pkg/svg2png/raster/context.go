package raster

import (
	"image"
	"image/color"
	"log"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/vector"

	"github.com/shinya/svg2png/pkg/svg2png/font"
	"github.com/shinya/svg2png/pkg/svg2png/parser"
	"github.com/shinya/svg2png/pkg/svg2png/style"
	"github.com/shinya/svg2png/pkg/svg2png/viewport"
)

// RasterContext は描画コンテキストを表します
type RasterContext struct {
	fb           *FrameBuffer
	fontRenderer *font.Renderer
	viewport     *viewport.Viewport
	defs         *parser.Defs
	clipMask     *image.Alpha
}

// NewRasterContext は新しいラスタリングコンテキストを作成します
func NewRasterContext(fb *FrameBuffer, fontRenderer *font.Renderer, vp *viewport.Viewport, defs *parser.Defs) *RasterContext {
	return &RasterContext{
		fb:           fb,
		fontRenderer: fontRenderer,
		viewport:     vp,
		defs:         defs,
	}
}

// ビューポートスケール情報
func (rc *RasterContext) scales() (scaleX, scaleY, offsetX, offsetY float64) {
	vb := rc.viewport.ViewBox
	scaleX = rc.viewport.Width / vb.Width
	scaleY = rc.viewport.Height / vb.Height
	offsetX = -vb.X * scaleX
	offsetY = -vb.Y * scaleY
	return
}

// toPixelXY はSVG座標をピクセル座標に変換します
func (rc *RasterContext) toPixelXY(x, y float64) (float32, float32) {
	scaleX, scaleY, offsetX, offsetY := rc.scales()
	return float32(x*scaleX + offsetX), float32(y*scaleY + offsetY)
}

// scaleLength は長さ値をスケーリングします（X方向）
func (rc *RasterContext) scaleLenX(v float64) float64 {
	scaleX, _, _, _ := rc.scales()
	return v * scaleX
}

// scaleLength は長さ値をスケーリングします（Y方向）
func (rc *RasterContext) scaleLenY(v float64) float64 {
	_, scaleY, _, _ := rc.scales()
	return v * scaleY
}

// fontScale はフォントスケールを返します（X/Y 平均）
func (rc *RasterContext) fontScale() float64 {
	scaleX, scaleY, _, _ := rc.scales()
	return (scaleX + scaleY) / 2.0
}

// scaledFontSizePt はOpenType points 単位のフォントサイズを返します
func (rc *RasterContext) scaledFontSizePt(st *style.ComputedStyle) float64 {
	dpi := rc.viewport.DPI
	if dpi == 0 {
		dpi = 96
	}
	return st.FontSize * rc.fontScale() * 72.0 / dpi
}

// fontStyleStr はComputedStyle からフォントスタイル文字列を返します
func (rc *RasterContext) fontStyleStr(st *style.ComputedStyle) string {
	fs := "Regular"
	if st.FontStyle == "italic" || st.FontStyle == "oblique" {
		fs = "Italic"
	}
	if st.FontWeight == "bold" || st.FontWeight == "700" || st.FontWeight == "800" || st.FontWeight == "900" {
		if fs == "Italic" {
			fs = "BoldItalic"
		} else {
			fs = "Bold"
		}
	}
	return fs
}

// fontFamilies はフォントファミリリストを返します（フォールバック含む）
func (rc *RasterContext) fontFamilies(st *style.ComputedStyle) []string {
	families := []string{st.FontFamily}
	switch strings.ToLower(st.FontFamily) {
	case "sans-serif", "helvetica", "arial":
		families = append(families, "Helvetica", "Arial", "Geneva", "FreeSans")
	case "serif", "times", "times new roman":
		families = append(families, "Times", "Times New Roman", "FreeSerif")
	case "monospace", "courier", "courier new":
		families = append(families, "Courier", "Courier New", "FreeMono")
	}
	return families
}

// measureTextPix はテキストのピクセル幅を返します（letter-spacing 込み）
func (rc *RasterContext) measureTextPix(content string, st *style.ComputedStyle) float64 {
	if rc.fontRenderer == nil || content == "" {
		return 0
	}
	scaledFontSize := rc.scaledFontSizePt(st)
	fontStyle := rc.fontStyleStr(st)
	families := rc.fontFamilies(st)

	for _, family := range families {
		if w, err := rc.fontRenderer.MeasureText(content, family, fontStyle, scaledFontSize); err == nil {
			nChars := float64(len([]rune(content)))
			w += st.LetterSpacing * rc.fontScale() * nChars
			return w
		}
	}
	return float64(len([]rune(content))) * scaledFontSize * 0.6
}

// drawTextRaw はテキストをピクセル位置に直接描画します（text-anchor 処理なし）
func (rc *RasterContext) drawTextRaw(content string, pixX, pixY float64, st *style.ComputedStyle) {
	if rc.fontRenderer == nil || content == "" {
		return
	}
	if st.FillNone {
		return
	}
	textColor := st.Fill

	scaledFontSize := rc.scaledFontSizePt(st)
	fontStyle := rc.fontStyleStr(st)
	families := rc.fontFamilies(st)

	// letter-spacing が指定されている場合は文字ごとに描画
	if st.LetterSpacing != 0 {
		rc.drawTextWithLetterSpacing(content, pixX, pixY, st, scaledFontSize, fontStyle, families, textColor)
		return
	}

	for _, family := range families {
		if err := rc.fontRenderer.RenderText(content, family, fontStyle, scaledFontSize, rc.fb.Image(), pixX, pixY, textColor); err == nil {
			return
		}
	}
	// フォールバック
	_ = rc.fontRenderer.RenderText(content, "", fontStyle, scaledFontSize, rc.fb.Image(), pixX, pixY, textColor)
}

// drawTextWithLetterSpacing は letter-spacing を考慮して1文字ずつ描画します
func (rc *RasterContext) drawTextWithLetterSpacing(content string, x, y float64, st *style.ComputedStyle, scaledFontSize float64, fontStyle string, families []string, textColor color.Color) {
	// 使用するフォントを決定
	usedFamily := ""
	for _, family := range families {
		if rc.fontRenderer.FindFont(family, fontStyle) != nil {
			usedFamily = family
			break
		}
	}

	letterSpacingPix := st.LetterSpacing * rc.fontScale()
	curX := x
	for _, r := range content {
		ch := string(r)
		_ = rc.fontRenderer.RenderText(ch, usedFamily, fontStyle, scaledFontSize, rc.fb.Image(), curX, y, textColor)
		// 文字幅を計測して進める
		advance := float64(len([]rune(ch))) * scaledFontSize * 0.6 // フォールバック幅
		if w, err := rc.fontRenderer.MeasureText(ch, usedFamily, fontStyle, scaledFontSize); err == nil {
			advance = w
		}
		curX += advance + letterSpacingPix
	}
}

// ============================================================
// クリップパス
// ============================================================

// PushClipPath は指定IDのclipPathをアクティブにします
func (rc *RasterContext) PushClipPath(clipPathID string) {
	if rc.defs == nil {
		return
	}
	clipElem, ok := rc.defs.ClipPaths[clipPathID]
	if !ok {
		log.Printf("clipPath not found: %s", clipPathID)
		return
	}

	w, h := rc.fb.Bounds().Dx(), rc.fb.Bounds().Dy()
	mask := image.NewAlpha(image.Rect(0, 0, w, h))

	for _, child := range clipElem.Children {
		rz := vector.NewRasterizer(w, h)
		built := false

		switch child.Name {
		case "polygon", "polyline":
			pts := parsePointsStrLocal(child.Attributes["points"])
			if len(pts) > 0 {
				x0, y0 := rc.toPixelXY(pts[0][0], pts[0][1])
				rz.MoveTo(x0, y0)
				for _, pt := range pts[1:] {
					px, py := rc.toPixelXY(pt[0], pt[1])
					rz.LineTo(px, py)
				}
				rz.ClosePath()
				built = true
			}
		case "rect":
			rx := parseAttrF(child, "x")
			ry := parseAttrF(child, "y")
			rw := parseAttrF(child, "width")
			rh := parseAttrF(child, "height")
			if rw > 0 && rh > 0 {
				x1, y1 := rc.toPixelXY(rx, ry)
				x2, y2 := rc.toPixelXY(rx+rw, ry+rh)
				addRoundedRect(rz, x1, y1, x2, y2, 0, 0)
				built = true
			}
		case "circle":
			cx := parseAttrF(child, "cx")
			cy := parseAttrF(child, "cy")
			r := parseAttrF(child, "r")
			if r > 0 {
				pcx, pcy := rc.toPixelXY(cx, cy)
				prx := float32(rc.scaleLenX(r))
				pry := float32(rc.scaleLenY(r))
				addEllipse(rz, pcx, pcy, prx, pry)
				built = true
			}
		case "path":
			d := child.Attributes["d"]
			if d != "" {
				toPixel := func(x, y float64) (float32, float32) { return rc.toPixelXY(x, y) }
				if err := buildPathRasterizer(rz, d, toPixel); err == nil {
					built = true
				}
			}
		}

		if built {
			rz.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
		}
	}

	rc.clipMask = mask
}

// PopClipPath はアクティブなclipPathを解除します
func (rc *RasterContext) PopClipPath() {
	rc.clipMask = nil
}

// applyClipToAlpha はalphaマスクにclipMaskを適用します
func (rc *RasterContext) applyClipToAlpha(alpha *image.Alpha) {
	if rc.clipMask == nil {
		return
	}
	bounds := alpha.Bounds()
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			a := alpha.AlphaAt(px, py).A
			if a == 0 {
				continue
			}
			c := rc.clipMask.AlphaAt(px, py).A
			alpha.SetAlpha(px, py, color.Alpha{A: uint8(uint16(a) * uint16(c) / 255)})
		}
	}
}

// compositeAlpha はアルファマスクを使って色をフレームバッファに合成します
func (rc *RasterContext) compositeAlpha(alpha *image.Alpha, col color.Color, opacity float64) {
	r16, g16, b16, _ := col.RGBA()
	cr := float64(r16 >> 8)
	cg := float64(g16 >> 8)
	cb := float64(b16 >> 8)

	img := rc.fb.Image()
	bounds := img.Bounds()

	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			mask := alpha.AlphaAt(px, py).A
			if mask == 0 {
				continue
			}

			// clipMask との AND
			if rc.clipMask != nil {
				cm := rc.clipMask.AlphaAt(px, py).A
				if cm == 0 {
					continue
				}
				mask = uint8(uint16(mask) * uint16(cm) / 255)
			}

			a := float64(mask) / 255.0 * opacity
			if a <= 0 {
				continue
			}

			bg := img.RGBAAt(px, py)
			newR := uint8(cr*a + float64(bg.R)*(1-a))
			newG := uint8(cg*a + float64(bg.G)*(1-a))
			newB := uint8(cb*a + float64(bg.B)*(1-a))
			newA := uint8(math.Min(255, float64(bg.A)+float64(mask)*opacity))

			img.SetRGBA(px, py, color.RGBA{newR, newG, newB, newA})
		}
	}
}

// rasterizeAndComposite はラスタライザーの内容を合成します
func (rc *RasterContext) rasterizeAndComposite(rz *vector.Rasterizer, col color.Color, opacity float64) {
	w, h := rc.fb.Bounds().Dx(), rc.fb.Bounds().Dy()
	alpha := image.NewAlpha(image.Rect(0, 0, w, h))
	rz.Draw(alpha, alpha.Bounds(), image.Opaque, image.Point{})
	rc.compositeAlpha(alpha, col, opacity)
}

// drawURLFill はFillURL に対応するグラデーション/パターン塗りを行います
func (rc *RasterContext) drawURLFill(alpha *image.Alpha, fillURL string, bounds image.Rectangle, opacity float64) {
	if rc.defs == nil {
		return
	}
	if lg, ok := rc.defs.LinearGradients[fillURL]; ok {
		rc.DrawLinearGradient(alpha, lg, bounds, opacity)
	} else if rg, ok := rc.defs.RadialGradients[fillURL]; ok {
		rc.DrawRadialGradient(alpha, rg, bounds, opacity)
	} else if pe, ok := rc.defs.Patterns[fillURL]; ok {
		rc.DrawPatternFill(alpha, pe, bounds, opacity, rc.defs)
	}
}

// ============================================================
// DrawRect
// ============================================================

// DrawRect は矩形を描画します
func (rc *RasterContext) DrawRect(rect *Rect, st *style.ComputedStyle) {
	log.Printf("DrawRect: x=%f y=%f w=%f h=%f", rect.X, rect.Y, rect.Width, rect.Height)

	x1, y1 := rc.toPixelXY(rect.X, rect.Y)
	x2, y2 := rc.toPixelXY(rect.X+rect.Width, rect.Y+rect.Height)

	rx := float32(rc.scaleLenX(rect.RX))
	ry := float32(rc.scaleLenY(rect.RY))

	w, h := rc.fb.Bounds().Dx(), rc.fb.Bounds().Dy()

	// Fill
	if st.FillURL != "" {
		rz := vector.NewRasterizer(w, h)
		addRoundedRect(rz, x1, y1, x2, y2, rx, ry)
		alpha := image.NewAlpha(image.Rect(0, 0, w, h))
		rz.Draw(alpha, alpha.Bounds(), image.Opaque, image.Point{})
		rc.applyClipToAlpha(alpha)
		bounds := image.Rect(int(x1), int(y1), int(x2), int(y2))
		rc.drawURLFill(alpha, st.FillURL, bounds, st.FillOpacity*st.Opacity)
	} else if !st.FillNone {
		_, _, _, fa := st.Fill.RGBA()
		if fa > 0 {
			rz := vector.NewRasterizer(w, h)
			addRoundedRect(rz, x1, y1, x2, y2, rx, ry)
			rc.rasterizeAndComposite(rz, st.Fill, st.FillOpacity*st.Opacity)
		}
	}

	// Stroke
	if !st.StrokeNone && st.StrokeWidth > 0 {
		_, _, _, sa := st.Stroke.RGBA()
		if sa > 0 {
			sw := float32(rc.scaleLenX(st.StrokeWidth))
			rz := vector.NewRasterizer(w, h)
			addStrokeRoundedRect(rz, x1, y1, x2, y2, rx, ry, sw)
			rc.rasterizeAndComposite(rz, st.Stroke, st.StrokeOpacity*st.Opacity)
		}
	}
}

// addRoundedRect はラスタライザーに角丸矩形パスを追加します
func addRoundedRect(rz *vector.Rasterizer, x1, y1, x2, y2, rx, ry float32) {
	if rx <= 0 || ry <= 0 {
		rz.MoveTo(x1, y1)
		rz.LineTo(x2, y1)
		rz.LineTo(x2, y2)
		rz.LineTo(x1, y2)
		rz.ClosePath()
		return
	}
	// 角丸矩形
	const k = 0.5522847498 // 4/3 * (sqrt(2)-1) ≈ bezier circle approximation
	rz.MoveTo(x1+rx, y1)
	rz.LineTo(x2-rx, y1)
	rz.CubeTo(x2-rx+k*rx, y1, x2, y1+ry-k*ry, x2, y1+ry)
	rz.LineTo(x2, y2-ry)
	rz.CubeTo(x2, y2-ry+k*ry, x2-rx+k*rx, y2, x2-rx, y2)
	rz.LineTo(x1+rx, y2)
	rz.CubeTo(x1+rx-k*rx, y2, x1, y2-ry+k*ry, x1, y2-ry)
	rz.LineTo(x1, y1+ry)
	rz.CubeTo(x1, y1+ry-k*ry, x1+rx-k*rx, y1, x1+rx, y1)
	rz.ClosePath()
}

// addStrokeRoundedRect はストローク用の角丸矩形（外側と内側の差）を追加します
func addStrokeRoundedRect(rz *vector.Rasterizer, x1, y1, x2, y2, rx, ry, sw float32) {
	half := sw / 2
	// 外側
	addRoundedRect(rz, x1-half, y1-half, x2+half, y2+half,
		maxF32(rx+half, 0), maxF32(ry+half, 0))
	// 内側（逆方向でくり抜き）
	innerX1, innerY1 := x1+half, y1+half
	innerX2, innerY2 := x2-half, y2-half
	if innerX2 > innerX1 && innerY2 > innerY1 {
		innerRX := maxF32(rx-half, 0)
		innerRY := maxF32(ry-half, 0)
		addRoundedRectCCW(rz, innerX1, innerY1, innerX2, innerY2, innerRX, innerRY)
	}
}

// addRoundedRectCCW は反時計回りの角丸矩形（くり抜き用）を追加します
func addRoundedRectCCW(rz *vector.Rasterizer, x1, y1, x2, y2, rx, ry float32) {
	if rx <= 0 || ry <= 0 {
		rz.MoveTo(x1, y1)
		rz.LineTo(x1, y2)
		rz.LineTo(x2, y2)
		rz.LineTo(x2, y1)
		rz.ClosePath()
		return
	}
	const k = float32(0.5522847498)
	rz.MoveTo(x1+rx, y1)
	rz.LineTo(x1, y1+ry)
	rz.CubeTo(x1, y1+ry-k*ry, x1+rx-k*rx, y1, x1+rx, y1)
	// 実際は反転が必要だが、vector パッケージは even-odd または nonzero rule で処理する
	// 非ゼロ規則でパスを逆にすることでくり抜きを実現
	rz.MoveTo(x1, y1)
	rz.LineTo(x1, y2)
	rz.LineTo(x2, y2)
	rz.LineTo(x2, y1)
	rz.ClosePath()
}

// ============================================================
// DrawCircle / DrawEllipse
// ============================================================

// DrawCircle は円を描画します
func (rc *RasterContext) DrawCircle(circle *Circle, st *style.ComputedStyle) {
	log.Printf("DrawCircle: cx=%f cy=%f r=%f", circle.CX, circle.CY, circle.R)
	rc.DrawEllipse(&Ellipse{CX: circle.CX, CY: circle.CY, RX: circle.R, RY: circle.R}, st)
}

// DrawEllipse は楕円を描画します
func (rc *RasterContext) DrawEllipse(ellipse *Ellipse, st *style.ComputedStyle) {
	cx, cy := rc.toPixelXY(ellipse.CX, ellipse.CY)
	rx := float32(rc.scaleLenX(ellipse.RX))
	ry := float32(rc.scaleLenY(ellipse.RY))
	w, h := rc.fb.Bounds().Dx(), rc.fb.Bounds().Dy()

	// Fill
	if st.FillURL != "" {
		rz := vector.NewRasterizer(w, h)
		addEllipse(rz, cx, cy, rx, ry)
		alpha := image.NewAlpha(image.Rect(0, 0, w, h))
		rz.Draw(alpha, alpha.Bounds(), image.Opaque, image.Point{})
		rc.applyClipToAlpha(alpha)
		bounds := image.Rect(int(cx-rx), int(cy-ry), int(cx+rx), int(cy+ry))
		rc.drawURLFill(alpha, st.FillURL, bounds, st.FillOpacity*st.Opacity)
	} else if !st.FillNone {
		_, _, _, fa := st.Fill.RGBA()
		if fa > 0 {
			rz := vector.NewRasterizer(w, h)
			addEllipse(rz, cx, cy, rx, ry)
			rc.rasterizeAndComposite(rz, st.Fill, st.FillOpacity*st.Opacity)
		}
	}

	// Stroke
	if !st.StrokeNone && st.StrokeWidth > 0 {
		_, _, _, sa := st.Stroke.RGBA()
		if sa > 0 {
			sw := float32(rc.scaleLenX(st.StrokeWidth))
			half := sw / 2
			if len(st.StrokeDasharray) > 0 {
				rc.strokeEllipseWithDash(cx, cy, rx, ry, sw, st.StrokeDasharray, st.Stroke, st.StrokeOpacity*st.Opacity)
			} else {
				rz := vector.NewRasterizer(w, h)
				// 外側楕円
				addEllipse(rz, cx, cy, rx+half, ry+half)
				// 内側楕円（反転でくり抜き）
				if rx > half && ry > half {
					addEllipseCCW(rz, cx, cy, rx-half, ry-half)
				}
				rc.rasterizeAndComposite(rz, st.Stroke, st.StrokeOpacity*st.Opacity)
			}
		}
	}
}

// strokeEllipseWithDash は破線で楕円ストロークを描画します
func (rc *RasterContext) strokeEllipseWithDash(cx, cy, rx, ry, sw float32, dasharray []float64, col color.Color, opacity float64) {
	// 楕円をN点でサンプリングしてポリラインに変換
	N := 360
	pts := make([][2]float32, N+1)
	for i := 0; i <= N; i++ {
		angle := float64(i) / float64(N) * 2 * math.Pi
		pts[i][0] = cx + rx*float32(math.Cos(angle))
		pts[i][1] = cy + ry*float32(math.Sin(angle))
	}

	w, h := rc.fb.Bounds().Dx(), rc.fb.Bounds().Dy()
	rz := vector.NewRasterizer(w, h)
	dashPixels := make([]float64, len(dasharray))
	for i, d := range dasharray {
		dashPixels[i] = d * rc.fontScale() // SVG→ピクセル
	}
	strokeSegmentsWithDash(rz, pts, sw, dashPixels)
	rc.rasterizeAndComposite(rz, col, opacity)
}

// addEllipse はラスタライザーに楕円パスを追加します（時計回り）
func addEllipse(rz *vector.Rasterizer, cx, cy, rx, ry float32) {
	const k = float32(0.5522847498)
	rz.MoveTo(cx+rx, cy)
	rz.CubeTo(cx+rx, cy+k*ry, cx+k*rx, cy+ry, cx, cy+ry)
	rz.CubeTo(cx-k*rx, cy+ry, cx-rx, cy+k*ry, cx-rx, cy)
	rz.CubeTo(cx-rx, cy-k*ry, cx-k*rx, cy-ry, cx, cy-ry)
	rz.CubeTo(cx+k*rx, cy-ry, cx+rx, cy-k*ry, cx+rx, cy)
	rz.ClosePath()
}

// addEllipseCCW はラスタライザーに楕円パスを追加します（反時計回り、くり抜き用）
func addEllipseCCW(rz *vector.Rasterizer, cx, cy, rx, ry float32) {
	const k = float32(0.5522847498)
	rz.MoveTo(cx+rx, cy)
	rz.CubeTo(cx+rx, cy-k*ry, cx+k*rx, cy-ry, cx, cy-ry)
	rz.CubeTo(cx-k*rx, cy-ry, cx-rx, cy-k*ry, cx-rx, cy)
	rz.CubeTo(cx-rx, cy+k*ry, cx-k*rx, cy+ry, cx, cy+ry)
	rz.CubeTo(cx+k*rx, cy+ry, cx+rx, cy+k*ry, cx+rx, cy)
	rz.ClosePath()
}

// ============================================================
// DrawLine
// ============================================================

// DrawLine は線を描画します
func (rc *RasterContext) DrawLine(line *Line, st *style.ComputedStyle) {
	if st.StrokeNone || st.StrokeWidth <= 0 {
		return
	}
	_, _, _, sa := st.Stroke.RGBA()
	if sa == 0 {
		return
	}

	x1, y1 := rc.toPixelXY(line.X1, line.Y1)
	x2, y2 := rc.toPixelXY(line.X2, line.Y2)
	sw := float32(rc.scaleLenX(st.StrokeWidth))

	w, h := rc.fb.Bounds().Dx(), rc.fb.Bounds().Dy()
	rz := vector.NewRasterizer(w, h)

	if len(st.StrokeDasharray) > 0 {
		dashPixels := scaleDasharray(st.StrokeDasharray, rc.fontScale())
		dashThickLine(rz, x1, y1, x2, y2, sw, dashPixels)
	} else {
		addThickLine(rz, x1, y1, x2, y2, sw)
	}
	rc.rasterizeAndComposite(rz, st.Stroke, st.StrokeOpacity*st.Opacity)
}

// addThickLine は太い線をラスタライザーに追加します
func addThickLine(rz *vector.Rasterizer, x1, y1, x2, y2, width float32) {
	dx := x2 - x1
	dy := y2 - y1
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length == 0 {
		return
	}
	// 法線方向
	nx := -dy / length * (width / 2)
	ny := dx / length * (width / 2)

	rz.MoveTo(x1+nx, y1+ny)
	rz.LineTo(x2+nx, y2+ny)
	rz.LineTo(x2-nx, y2-ny)
	rz.LineTo(x1-nx, y1-ny)
	rz.ClosePath()
}

// scaleDasharray は SVG単位の dasharray をピクセル単位に変換します
func scaleDasharray(dasharray []float64, scale float64) []float64 {
	result := make([]float64, len(dasharray))
	for i, d := range dasharray {
		result[i] = d * scale
	}
	return result
}

// dashThickLine は破線の太い線をラスタライザーに追加します
func dashThickLine(rz *vector.Rasterizer, x1, y1, x2, y2, width float32, dashPx []float64) {
	dx := x2 - x1
	dy := y2 - y1
	totalLen := float64(math.Sqrt(float64(dx*dx + dy*dy)))
	if totalLen == 0 {
		return
	}
	ux := float64(dx) / totalLen
	uy := float64(dy) / totalLen

	pos := 0.0
	dashIdx := 0
	drawing := true

	for pos < totalLen {
		dashLen := dashPx[dashIdx%len(dashPx)]
		endPos := pos + dashLen
		if endPos > totalLen {
			endPos = totalLen
		}
		if drawing {
			sx := float32(float64(x1) + ux*pos)
			sy := float32(float64(y1) + uy*pos)
			ex := float32(float64(x1) + ux*endPos)
			ey := float32(float64(y1) + uy*endPos)
			addThickLine(rz, sx, sy, ex, ey, width)
		}
		pos = endPos
		dashIdx++
		drawing = !drawing
	}
}

// strokeSegmentsWithDash はピクセル座標セグメントに dasharray を適用します
func strokeSegmentsWithDash(rz *vector.Rasterizer, pts [][2]float32, sw float32, dashPx []float64) {
	if len(pts) < 2 {
		return
	}
	dashIdx := 0
	dashPos := 0.0
	drawing := true

	for i := 0; i < len(pts)-1; i++ {
		x1, y1 := pts[i][0], pts[i][1]
		x2, y2 := pts[i+1][0], pts[i+1][1]
		dx, dy := x2-x1, y2-y1
		segLen := float64(math.Sqrt(float64(dx*dx + dy*dy)))
		if segLen == 0 {
			continue
		}
		ux, uy := float64(dx)/segLen, float64(dy)/segLen
		pos := 0.0
		for pos < segLen {
			remaining := dashPx[dashIdx%len(dashPx)] - dashPos
			advance := math.Min(remaining, segLen-pos)
			if drawing {
				sx := float32(float64(x1) + ux*pos)
				sy := float32(float64(y1) + uy*pos)
				ex := float32(float64(x1) + ux*(pos+advance))
				ey := float32(float64(y1) + uy*(pos+advance))
				addThickLine(rz, sx, sy, ex, ey, sw)
			}
			pos += advance
			dashPos += advance
			if dashPos >= dashPx[dashIdx%len(dashPx)] {
				dashPos = 0
				dashIdx++
				drawing = !drawing
			}
		}
	}
}

// ============================================================
// DrawPolyline / DrawPolygon
// ============================================================

// DrawPolyline は折れ線を描画します
func (rc *RasterContext) DrawPolyline(points []Point, st *style.ComputedStyle, closed bool) {
	if len(points) < 2 {
		return
	}

	w, h := rc.fb.Bounds().Dx(), rc.fb.Bounds().Dy()

	// Fill（polygon のみ）
	if closed && !st.FillNone {
		_, _, _, fa := st.Fill.RGBA()
		if fa > 0 {
			rz := vector.NewRasterizer(w, h)
			x0, y0 := rc.toPixelXY(points[0].X, points[0].Y)
			rz.MoveTo(x0, y0)
			for _, p := range points[1:] {
				px, py := rc.toPixelXY(p.X, p.Y)
				rz.LineTo(px, py)
			}
			rz.ClosePath()
			rc.rasterizeAndComposite(rz, st.Fill, st.FillOpacity*st.Opacity)
		}
	}

	// Stroke
	if !st.StrokeNone && st.StrokeWidth > 0 {
		_, _, _, sa := st.Stroke.RGBA()
		if sa > 0 {
			sw := float32(rc.scaleLenX(st.StrokeWidth))
			if len(st.StrokeDasharray) > 0 {
				// 破線ストローク
				dashPixels := scaleDasharray(st.StrokeDasharray, rc.fontScale())
				pts := make([][2]float32, len(points))
				for i, p := range points {
					pts[i][0], pts[i][1] = rc.toPixelXY(p.X, p.Y)
				}
				if closed {
					pts = append(pts, pts[0])
				}
				rz := vector.NewRasterizer(w, h)
				strokeSegmentsWithDash(rz, pts, sw, dashPixels)
				rc.rasterizeAndComposite(rz, st.Stroke, st.StrokeOpacity*st.Opacity)
			} else {
				rz := vector.NewRasterizer(w, h)
				for i := 0; i < len(points)-1; i++ {
					x1, y1 := rc.toPixelXY(points[i].X, points[i].Y)
					x2, y2 := rc.toPixelXY(points[i+1].X, points[i+1].Y)
					addThickLine(rz, x1, y1, x2, y2, sw)
				}
				if closed {
					x1, y1 := rc.toPixelXY(points[len(points)-1].X, points[len(points)-1].Y)
					x2, y2 := rc.toPixelXY(points[0].X, points[0].Y)
					addThickLine(rz, x1, y1, x2, y2, sw)
				}
				rc.rasterizeAndComposite(rz, st.Stroke, st.StrokeOpacity*st.Opacity)
			}
		}
	}
}

// ============================================================
// DrawPath
// ============================================================

// DrawPath はSVGパスを描画します
func (rc *RasterContext) DrawPath(path *Path, st *style.ComputedStyle) {
	log.Printf("DrawPath: d=%s", path.Data)
	if path.Data == "" {
		return
	}

	scaleX, scaleY, offsetX, offsetY := rc.scales()
	w, h := rc.fb.Bounds().Dx(), rc.fb.Bounds().Dy()

	toPixel := func(x, y float64) (float32, float32) {
		return float32(x*scaleX + offsetX), float32(y*scaleY + offsetY)
	}
	scaleLX := func(v float64) float64 { return v * scaleX }

	// Fill
	if st.FillURL != "" {
		rz := vector.NewRasterizer(w, h)
		if err := buildPathRasterizer(rz, path.Data, toPixel); err == nil {
			alpha := image.NewAlpha(image.Rect(0, 0, w, h))
			rz.Draw(alpha, alpha.Bounds(), image.Opaque, image.Point{})
			rc.applyClipToAlpha(alpha)
			// パスのboundsは全キャンバスを使用（大抵の場合は問題ない）
			bounds := rc.fb.Bounds()
			rc.drawURLFill(alpha, st.FillURL, bounds, st.FillOpacity*st.Opacity)
		}
	} else if !st.FillNone {
		_, _, _, fa := st.Fill.RGBA()
		if fa > 0 {
			rz := vector.NewRasterizer(w, h)
			if err := buildPathRasterizer(rz, path.Data, toPixel); err != nil {
				log.Printf("DrawPath fill error: %v", err)
			} else {
				rc.rasterizeAndComposite(rz, st.Fill, st.FillOpacity*st.Opacity)
			}
		}
	}

	// Stroke
	if !st.StrokeNone && st.StrokeWidth > 0 {
		_, _, _, sa := st.Stroke.RGBA()
		if sa > 0 {
			sw := float32(scaleLX(st.StrokeWidth))
			if len(st.StrokeDasharray) > 0 {
				// パスを線分に分解してdashで描く
				dashPixels := scaleDasharray(st.StrokeDasharray, rc.fontScale())
				rz := vector.NewRasterizer(w, h)
				buildDashStrokeFromPath(rz, path.Data, sw, toPixel, dashPixels)
				rc.rasterizeAndComposite(rz, st.Stroke, st.StrokeOpacity*st.Opacity)
			} else {
				rz := vector.NewRasterizer(w, h)
				if err := buildStrokeRasterizer(rz, path.Data, sw, toPixel); err != nil {
					log.Printf("DrawPath stroke error: %v", err)
				} else {
					rc.rasterizeAndComposite(rz, st.Stroke, st.StrokeOpacity*st.Opacity)
				}
			}
		}
	}
}

// buildDashStrokeFromPath はパスの各セグメントを破線ストロークでレンダリングします
func buildDashStrokeFromPath(rz *vector.Rasterizer, data string, sw float32, toPixel func(float64, float64) (float32, float32), dashPx []float64) {
	// パスを線分に展開してからdashを適用する
	type seg struct{ x1, y1, x2, y2 float32 }
	var segs []seg

	var curX, curY, startX, startY float64
	pr := &pathReader{s: data}
	var lastCmd byte

	for !pr.done() {
		pr.skipWS()
		if pr.done() {
			break
		}
		c := pr.s[pr.pos]
		if isPathCmd(c) {
			lastCmd = c
			pr.pos++
		}

		prevX, prevY := curX, curY

		switch lastCmd {
		case 'M':
			x, ok1 := pr.readFloat()
			y, ok2 := pr.readFloat()
			if !ok1 || !ok2 {
				goto nextCmd2
			}
			curX, curY = x, y
			startX, startY = x, y
			lastCmd = 'L'
		case 'm':
			dx, ok1 := pr.readFloat()
			dy, ok2 := pr.readFloat()
			if !ok1 || !ok2 {
				goto nextCmd2
			}
			curX += dx
			curY += dy
			startX, startY = curX, curY
			lastCmd = 'l'
		case 'L':
			x, ok1 := pr.readFloat()
			y, ok2 := pr.readFloat()
			if !ok1 || !ok2 {
				goto nextCmd2
			}
			px1, py1 := toPixel(prevX, prevY)
			px2, py2 := toPixel(x, y)
			segs = append(segs, seg{px1, py1, px2, py2})
			curX, curY = x, y
		case 'l':
			dx, ok1 := pr.readFloat()
			dy, ok2 := pr.readFloat()
			if !ok1 || !ok2 {
				goto nextCmd2
			}
			nx, ny := curX+dx, curY+dy
			px1, py1 := toPixel(curX, curY)
			px2, py2 := toPixel(nx, ny)
			segs = append(segs, seg{px1, py1, px2, py2})
			curX, curY = nx, ny
		case 'Z', 'z':
			px1, py1 := toPixel(curX, curY)
			px2, py2 := toPixel(startX, startY)
			segs = append(segs, seg{px1, py1, px2, py2})
			curX, curY = startX, startY
		default:
			for pr.isNextFloat() {
				pr.readFloat()
			}
		}
		continue
	nextCmd2:
		_ = prevX
		_ = prevY
	}

	dashIdx := 0
	dashPos := 0.0
	drawing := true
	for _, s := range segs {
		dx, dy := s.x2-s.x1, s.y2-s.y1
		segLen := float64(math.Sqrt(float64(dx*dx + dy*dy)))
		if segLen == 0 {
			continue
		}
		ux, uy := float64(dx)/segLen, float64(dy)/segLen
		pos := 0.0
		for pos < segLen {
			remaining := dashPx[dashIdx%len(dashPx)] - dashPos
			advance := math.Min(remaining, segLen-pos)
			if drawing {
				sx := float32(float64(s.x1) + ux*pos)
				sy := float32(float64(s.y1) + uy*pos)
				ex := float32(float64(s.x1) + ux*(pos+advance))
				ey := float32(float64(s.y1) + uy*(pos+advance))
				addThickLine(rz, sx, sy, ex, ey, sw)
			}
			pos += advance
			dashPos += advance
			if dashPos >= dashPx[dashIdx%len(dashPx)] {
				dashPos = 0
				dashIdx++
				drawing = !drawing
			}
		}
	}
}

// ============================================================
// SVG パスパーサー
// ============================================================

// pathReader はSVGパスデータを読み取るためのリーダーです
type pathReader struct {
	s   string
	pos int
}

func (p *pathReader) done() bool { return p.pos >= len(p.s) }

func (p *pathReader) skipWS() {
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' {
			p.pos++
		} else {
			break
		}
	}
}

func (p *pathReader) readFloat() (float64, bool) {
	p.skipWS()
	if p.done() {
		return 0, false
	}
	start := p.pos

	// 符号
	if p.s[p.pos] == '-' || p.s[p.pos] == '+' {
		p.pos++
	}

	// 整数部
	hasDigit := false
	for !p.done() && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
		p.pos++
		hasDigit = true
	}

	// 小数部
	if !p.done() && p.s[p.pos] == '.' {
		p.pos++
		for !p.done() && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
			p.pos++
			hasDigit = true
		}
	}

	if !hasDigit {
		p.pos = start
		return 0, false
	}

	// 指数部
	if !p.done() && (p.s[p.pos] == 'e' || p.s[p.pos] == 'E') {
		p.pos++
		if !p.done() && (p.s[p.pos] == '-' || p.s[p.pos] == '+') {
			p.pos++
		}
		for !p.done() && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
			p.pos++
		}
	}

	v, err := strconv.ParseFloat(p.s[start:p.pos], 64)
	if err != nil {
		p.pos = start
		return 0, false
	}
	return v, true
}

func (p *pathReader) isNextFloat() bool {
	p.skipWS()
	if p.done() {
		return false
	}
	c := p.s[p.pos]
	return c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9')
}

func isPathCmd(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// buildPathRasterizer はSVGパスをラスタライザーに追加します
func buildPathRasterizer(rz *vector.Rasterizer, data string, toPixel func(float64, float64) (float32, float32)) error {
	return buildPath(rz, data, toPixel, false, 0)
}

// buildStrokeRasterizer はSVGパスのストロークをラスタライザーに追加します
func buildStrokeRasterizer(rz *vector.Rasterizer, data string, strokeWidth float32, toPixel func(float64, float64) (float32, float32)) error {
	// パスコマンドをセグメントとして収集してストロークを展開する
	type seg struct{ x1, y1, x2, y2 float32 }
	var segs []seg

	var curX, curY, startX, startY float64
	pr := &pathReader{s: data}
	var lastCmd byte

	for !pr.done() {
		pr.skipWS()
		if pr.done() {
			break
		}

		c := pr.s[pr.pos]
		if isPathCmd(c) {
			lastCmd = c
			pr.pos++
			if lastCmd == 'M' {
				lastCmd = 'M'
			} else if lastCmd == 'm' {
				lastCmd = 'm'
			}
		}

		prevX, prevY := curX, curY

		switch lastCmd {
		case 'M':
			x, ok1 := pr.readFloat()
			y, ok2 := pr.readFloat()
			if !ok1 || !ok2 {
				goto nextCmd
			}
			curX, curY = x, y
			startX, startY = x, y
			lastCmd = 'L'
		case 'm':
			dx, ok1 := pr.readFloat()
			dy, ok2 := pr.readFloat()
			if !ok1 || !ok2 {
				goto nextCmd
			}
			curX += dx
			curY += dy
			startX, startY = curX, curY
			lastCmd = 'l'
		case 'L':
			x, ok1 := pr.readFloat()
			y, ok2 := pr.readFloat()
			if !ok1 || !ok2 {
				goto nextCmd
			}
			px1, py1 := toPixel(prevX, prevY)
			px2, py2 := toPixel(x, y)
			segs = append(segs, seg{px1, py1, px2, py2})
			curX, curY = x, y
		case 'l':
			dx, ok1 := pr.readFloat()
			dy, ok2 := pr.readFloat()
			if !ok1 || !ok2 {
				goto nextCmd
			}
			nx, ny := curX+dx, curY+dy
			px1, py1 := toPixel(curX, curY)
			px2, py2 := toPixel(nx, ny)
			segs = append(segs, seg{px1, py1, px2, py2})
			curX, curY = nx, ny
		case 'H':
			x, ok := pr.readFloat()
			if !ok {
				goto nextCmd
			}
			px1, py1 := toPixel(curX, curY)
			px2, py2 := toPixel(x, curY)
			segs = append(segs, seg{px1, py1, px2, py2})
			curX = x
		case 'h':
			dx, ok := pr.readFloat()
			if !ok {
				goto nextCmd
			}
			nx := curX + dx
			px1, py1 := toPixel(curX, curY)
			px2, py2 := toPixel(nx, curY)
			segs = append(segs, seg{px1, py1, px2, py2})
			curX = nx
		case 'V':
			y, ok := pr.readFloat()
			if !ok {
				goto nextCmd
			}
			px1, py1 := toPixel(curX, curY)
			px2, py2 := toPixel(curX, y)
			segs = append(segs, seg{px1, py1, px2, py2})
			curY = y
		case 'v':
			dy, ok := pr.readFloat()
			if !ok {
				goto nextCmd
			}
			ny := curY + dy
			px1, py1 := toPixel(curX, curY)
			px2, py2 := toPixel(curX, ny)
			segs = append(segs, seg{px1, py1, px2, py2})
			curY = ny
		case 'Z', 'z':
			px1, py1 := toPixel(curX, curY)
			px2, py2 := toPixel(startX, startY)
			segs = append(segs, seg{px1, py1, px2, py2})
			curX, curY = startX, startY
		default:
			// その他のコマンド（C, Q など）はエンドポイントのみ使う（近似）
			for pr.isNextFloat() {
				pr.readFloat()
			}
		}
		continue
	nextCmd:
		_ = prevX
		_ = prevY
	}

	for _, s := range segs {
		addThickLine(rz, s.x1, s.y1, s.x2, s.y2, strokeWidth)
	}
	return nil
}

// buildPath はSVGパスデータをラスタライザーに追加します（fill用）
func buildPath(rz *vector.Rasterizer, data string, toPixel func(float64, float64) (float32, float32), _ bool, _ float32) error {
	pr := &pathReader{s: data}

	var curX, curY float64     // 現在位置
	var startX, startY float64 // サブパス開始位置
	var lastCP [2]float64      // 最後のコントロールポイント（S/T用）
	var lastCmd byte

	for !pr.done() {
		pr.skipWS()
		if pr.done() {
			break
		}

		c := pr.s[pr.pos]
		if isPathCmd(c) {
			lastCmd = c
			pr.pos++
		}

		switch lastCmd {
		case 'M', 'm':
			x, ok1 := pr.readFloat()
			y, ok2 := pr.readFloat()
			if !ok1 || !ok2 {
				break
			}
			if lastCmd == 'm' {
				x += curX
				y += curY
			}
			curX, curY = x, y
			startX, startY = x, y
			px, py := toPixel(x, y)
			rz.MoveTo(px, py)
			// 後続の座標ペアは LineTo として扱う
			if lastCmd == 'M' {
				lastCmd = 'L'
			} else {
				lastCmd = 'l'
			}

		case 'L':
			for pr.isNextFloat() {
				x, ok1 := pr.readFloat()
				y, ok2 := pr.readFloat()
				if !ok1 || !ok2 {
					break
				}
				curX, curY = x, y
				px, py := toPixel(x, y)
				rz.LineTo(px, py)
			}
		case 'l':
			for pr.isNextFloat() {
				dx, ok1 := pr.readFloat()
				dy, ok2 := pr.readFloat()
				if !ok1 || !ok2 {
					break
				}
				curX += dx
				curY += dy
				px, py := toPixel(curX, curY)
				rz.LineTo(px, py)
			}

		case 'H':
			for pr.isNextFloat() {
				x, ok := pr.readFloat()
				if !ok {
					break
				}
				curX = x
				px, py := toPixel(curX, curY)
				rz.LineTo(px, py)
			}
		case 'h':
			for pr.isNextFloat() {
				dx, ok := pr.readFloat()
				if !ok {
					break
				}
				curX += dx
				px, py := toPixel(curX, curY)
				rz.LineTo(px, py)
			}

		case 'V':
			for pr.isNextFloat() {
				y, ok := pr.readFloat()
				if !ok {
					break
				}
				curY = y
				px, py := toPixel(curX, curY)
				rz.LineTo(px, py)
			}
		case 'v':
			for pr.isNextFloat() {
				dy, ok := pr.readFloat()
				if !ok {
					break
				}
				curY += dy
				px, py := toPixel(curX, curY)
				rz.LineTo(px, py)
			}

		case 'C':
			for pr.isNextFloat() {
				x1, ok1 := pr.readFloat()
				y1, ok2 := pr.readFloat()
				x2, ok3 := pr.readFloat()
				y2, ok4 := pr.readFloat()
				x, ok5 := pr.readFloat()
				y, ok6 := pr.readFloat()
				if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
					break
				}
				lastCP = [2]float64{x2, y2}
				curX, curY = x, y
				p1x, p1y := toPixel(x1, y1)
				p2x, p2y := toPixel(x2, y2)
				pex, pey := toPixel(x, y)
				rz.CubeTo(p1x, p1y, p2x, p2y, pex, pey)
			}
		case 'c':
			for pr.isNextFloat() {
				dx1, ok1 := pr.readFloat()
				dy1, ok2 := pr.readFloat()
				dx2, ok3 := pr.readFloat()
				dy2, ok4 := pr.readFloat()
				dx, ok5 := pr.readFloat()
				dy, ok6 := pr.readFloat()
				if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
					break
				}
				x1, y1 := curX+dx1, curY+dy1
				x2, y2 := curX+dx2, curY+dy2
				x, y := curX+dx, curY+dy
				lastCP = [2]float64{x2, y2}
				curX, curY = x, y
				p1x, p1y := toPixel(x1, y1)
				p2x, p2y := toPixel(x2, y2)
				pex, pey := toPixel(x, y)
				rz.CubeTo(p1x, p1y, p2x, p2y, pex, pey)
			}

		case 'S':
			for pr.isNextFloat() {
				x2, ok1 := pr.readFloat()
				y2, ok2 := pr.readFloat()
				x, ok3 := pr.readFloat()
				y, ok4 := pr.readFloat()
				if !ok1 || !ok2 || !ok3 || !ok4 {
					break
				}
				// 前のコントロールポイントの反射
				x1 := 2*curX - lastCP[0]
				y1 := 2*curY - lastCP[1]
				lastCP = [2]float64{x2, y2}
				curX, curY = x, y
				p1x, p1y := toPixel(x1, y1)
				p2x, p2y := toPixel(x2, y2)
				pex, pey := toPixel(x, y)
				rz.CubeTo(p1x, p1y, p2x, p2y, pex, pey)
			}
		case 's':
			for pr.isNextFloat() {
				dx2, ok1 := pr.readFloat()
				dy2, ok2 := pr.readFloat()
				dx, ok3 := pr.readFloat()
				dy, ok4 := pr.readFloat()
				if !ok1 || !ok2 || !ok3 || !ok4 {
					break
				}
				x1 := 2*curX - lastCP[0]
				y1 := 2*curY - lastCP[1]
				x2, y2 := curX+dx2, curY+dy2
				x, y := curX+dx, curY+dy
				lastCP = [2]float64{x2, y2}
				curX, curY = x, y
				p1x, p1y := toPixel(x1, y1)
				p2x, p2y := toPixel(x2, y2)
				pex, pey := toPixel(x, y)
				rz.CubeTo(p1x, p1y, p2x, p2y, pex, pey)
			}

		case 'Q':
			for pr.isNextFloat() {
				x1, ok1 := pr.readFloat()
				y1, ok2 := pr.readFloat()
				x, ok3 := pr.readFloat()
				y, ok4 := pr.readFloat()
				if !ok1 || !ok2 || !ok3 || !ok4 {
					break
				}
				lastCP = [2]float64{x1, y1}
				curX, curY = x, y
				p1x, p1y := toPixel(x1, y1)
				pex, pey := toPixel(x, y)
				rz.QuadTo(p1x, p1y, pex, pey)
			}
		case 'q':
			for pr.isNextFloat() {
				dx1, ok1 := pr.readFloat()
				dy1, ok2 := pr.readFloat()
				dx, ok3 := pr.readFloat()
				dy, ok4 := pr.readFloat()
				if !ok1 || !ok2 || !ok3 || !ok4 {
					break
				}
				x1, y1 := curX+dx1, curY+dy1
				x, y := curX+dx, curY+dy
				lastCP = [2]float64{x1, y1}
				curX, curY = x, y
				p1x, p1y := toPixel(x1, y1)
				pex, pey := toPixel(x, y)
				rz.QuadTo(p1x, p1y, pex, pey)
			}

		case 'T':
			for pr.isNextFloat() {
				x, ok1 := pr.readFloat()
				y, ok2 := pr.readFloat()
				if !ok1 || !ok2 {
					break
				}
				x1 := 2*curX - lastCP[0]
				y1 := 2*curY - lastCP[1]
				lastCP = [2]float64{x1, y1}
				curX, curY = x, y
				p1x, p1y := toPixel(x1, y1)
				pex, pey := toPixel(x, y)
				rz.QuadTo(p1x, p1y, pex, pey)
			}
		case 't':
			for pr.isNextFloat() {
				dx, ok1 := pr.readFloat()
				dy, ok2 := pr.readFloat()
				if !ok1 || !ok2 {
					break
				}
				x1 := 2*curX - lastCP[0]
				y1 := 2*curY - lastCP[1]
				x, y := curX+dx, curY+dy
				lastCP = [2]float64{x1, y1}
				curX, curY = x, y
				p1x, p1y := toPixel(x1, y1)
				pex, pey := toPixel(x, y)
				rz.QuadTo(p1x, p1y, pex, pey)
			}

		case 'A', 'a':
			for pr.isNextFloat() {
				rx, ok1 := pr.readFloat()
				ry, ok2 := pr.readFloat()
				xRot, ok3 := pr.readFloat()
				largeArcF, ok4 := pr.readFloat()
				sweepF, ok5 := pr.readFloat()
				xe, ok6 := pr.readFloat()
				ye, ok7 := pr.readFloat()
				if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 || !ok7 {
					break
				}
				if lastCmd == 'a' {
					xe += curX
					ye += curY
				}
				arcToBezier(rz, curX, curY, rx, ry, xRot, largeArcF != 0, sweepF != 0, xe, ye, toPixel)
				curX, curY = xe, ye
			}

		case 'Z', 'z':
			rz.ClosePath()
			curX, curY = startX, startY
		}
	}

	return nil
}

// arcToBezier はSVG楕円弧をcubic Bezier曲線に変換してラスタライザーに追加します
func arcToBezier(rz *vector.Rasterizer, x1, y1, rx, ry, phi float64, largeArc, sweep bool, x2, y2 float64, toPixel func(float64, float64) (float32, float32)) {
	if rx == 0 || ry == 0 {
		px, py := toPixel(x2, y2)
		rz.LineTo(px, py)
		return
	}
	if x1 == x2 && y1 == y2 {
		return
	}

	rx = math.Abs(rx)
	ry = math.Abs(ry)

	phiRad := phi * math.Pi / 180
	cosPhi := math.Cos(phiRad)
	sinPhi := math.Sin(phiRad)

	// Step 1: (x1', y1')
	dx := (x1 - x2) / 2
	dy := (y1 - y2) / 2
	x1p := cosPhi*dx + sinPhi*dy
	y1p := -sinPhi*dx + cosPhi*dy

	// Step 2: ラジアスの調整と中心点の計算
	x1psq := x1p * x1p
	y1psq := y1p * y1p
	rxsq := rx * rx
	rysq := ry * ry

	lambda := x1psq/rxsq + y1psq/rysq
	if lambda > 1 {
		sqrtL := math.Sqrt(lambda)
		rx *= sqrtL
		ry *= sqrtL
		rxsq = rx * rx
		rysq = ry * ry
	}

	num := rxsq*rysq - rxsq*y1psq - rysq*x1psq
	den := rxsq*y1psq + rysq*x1psq

	sq := 0.0
	if den > 0 && num/den > 0 {
		sq = math.Sqrt(num / den)
	}
	if largeArc == sweep {
		sq = -sq
	}

	cxp := sq * rx * y1p / ry
	cyp := -sq * ry * x1p / rx

	// Step 3: 中心点
	cx := cosPhi*cxp - sinPhi*cyp + (x1+x2)/2
	cy := sinPhi*cxp + cosPhi*cyp + (y1+y2)/2

	// Step 4: 角度の計算
	ux := (x1p - cxp) / rx
	uy := (y1p - cyp) / ry
	vx := (-x1p - cxp) / rx
	vy := (-y1p - cyp) / ry

	// theta1
	theta1 := math.Atan2(uy, ux)

	// dtheta
	dot := ux*vx + uy*vy
	dot = math.Max(-1, math.Min(1, dot))
	dtheta := math.Acos(dot)
	if ux*vy-uy*vx < 0 {
		dtheta = -dtheta
	}

	if sweep && dtheta < 0 {
		dtheta += 2 * math.Pi
	} else if !sweep && dtheta > 0 {
		dtheta -= 2 * math.Pi
	}

	// 弧をセグメントに分割
	nSegs := int(math.Ceil(math.Abs(dtheta) / (math.Pi / 2)))
	if nSegs == 0 {
		nSegs = 1
	}

	dthetaSeg := dtheta / float64(nSegs)

	for i := 0; i < nSegs; i++ {
		t1 := theta1 + float64(i)*dthetaSeg
		t2 := theta1 + float64(i+1)*dthetaSeg

		alpha := math.Sin(t2-t1) * (math.Sqrt(4+3*math.Pow(math.Tan((t2-t1)/2), 2)) - 1) / 3

		// 開始・終了点と導関数
		cos1, sin1 := math.Cos(t1), math.Sin(t1)
		cos2, sin2 := math.Cos(t2), math.Sin(t2)

		// 楕円上の点と導関数
		ex1 := cx + rx*cosPhi*cos1 - ry*sinPhi*sin1
		ey1 := cy + rx*sinPhi*cos1 + ry*cosPhi*sin1
		dex1 := -rx*cosPhi*sin1 - ry*sinPhi*cos1
		dey1 := -rx*sinPhi*sin1 + ry*cosPhi*cos1

		ex2 := cx + rx*cosPhi*cos2 - ry*sinPhi*sin2
		ey2 := cy + rx*sinPhi*cos2 + ry*cosPhi*sin2
		dex2 := -rx*cosPhi*sin2 - ry*sinPhi*cos2
		dey2 := -rx*sinPhi*sin2 + ry*cosPhi*cos2

		cp1x := ex1 + alpha*dex1
		cp1y := ey1 + alpha*dey1
		cp2x := ex2 - alpha*dex2
		cp2y := ey2 - alpha*dey2

		_ = ex1
		_ = ey1

		pcp1x, pcp1y := toPixel(cp1x, cp1y)
		pcp2x, pcp2y := toPixel(cp2x, cp2y)
		pex, pey := toPixel(ex2, ey2)
		rz.CubeTo(pcp1x, pcp1y, pcp2x, pcp2y, pex, pey)
	}
}

// ============================================================
// DrawText
// ============================================================

// TextSpan はテキストスパン（テキスト＋スタイル）を表します
type TextSpan struct {
	Content string
	Style   *style.ComputedStyle
}

// DrawTextGroup は複数スパンをグループとして描画します（text-anchor対応）
func (rc *RasterContext) DrawTextGroup(spans []TextSpan, anchorX, anchorY float64, textAnchor string) {
	if len(spans) == 0 || rc.fontRenderer == nil {
		return
	}

	_, py := rc.toPixelXY(anchorX, anchorY)
	px, _ := rc.toPixelXY(anchorX, anchorY)

	// 各スパンの幅を計測
	widths := make([]float64, len(spans))
	totalWidth := 0.0
	for i, s := range spans {
		if s.Style.FillNone {
			continue
		}
		w := rc.measureTextPix(s.Content, s.Style)
		widths[i] = w
		totalWidth += w
	}

	// text-anchor に基づいて開始x を決定
	startX := float64(px)
	switch textAnchor {
	case "middle":
		startX -= totalWidth / 2
	case "end":
		startX -= totalWidth
	}

	// スパンを順に描画
	curX := startX
	for i, s := range spans {
		if s.Content == "" || s.Style.FillNone {
			curX += widths[i]
			continue
		}
		rc.drawTextRaw(s.Content, curX, float64(py), s.Style)
		curX += widths[i]
	}
}

// DrawText はテキストを描画します
func (rc *RasterContext) DrawText(text *Text, st *style.ComputedStyle) {
	log.Printf("DrawText: x=%f y=%f content='%s'", text.X, text.Y, text.Content)

	if rc.fontRenderer == nil {
		log.Printf("Font renderer is nil")
		return
	}
	if text.Content == "" {
		return
	}

	// ビューポート変換
	px, py := rc.toPixelXY(text.X, text.Y)

	textColor := st.Fill
	if st.FillNone {
		return // fill=none のテキストは見えない
	}

	// テキスト幅を計測（letter-spacing 込み）
	textWidth := rc.measureTextPix(text.Content, st)

	// text-anchor に基づいて x 位置を調整
	finalX := float64(px)
	switch st.TextAnchor {
	case "middle":
		finalX -= textWidth / 2
	case "end":
		finalX -= textWidth
	}

	// テキストを描画
	scaledFontSize := rc.scaledFontSizePt(st)
	fontStyle := rc.fontStyleStr(st)
	families := rc.fontFamilies(st)

	if st.LetterSpacing != 0 {
		rc.drawTextWithLetterSpacing(text.Content, finalX, float64(py), st, scaledFontSize, fontStyle, families, textColor)
		return
	}

	for _, family := range families {
		err := rc.fontRenderer.RenderText(text.Content, family, fontStyle, scaledFontSize, rc.fb.Image(), finalX, float64(py), textColor)
		if err == nil {
			log.Printf("Text rendered with font %s", family)
			return
		}
	}

	// 最終フォールバック: basicfont
	_ = rc.fontRenderer.RenderText(text.Content, "", fontStyle, scaledFontSize, rc.fb.Image(), finalX, float64(py), textColor)
	log.Printf("Text rendered with basicfont fallback for '%s'", text.Content)
}

// ============================================================
// 型定義
// ============================================================

// Path はパス要素を表します
type Path struct {
	Data string
}

// Rect は矩形要素を表します
type Rect struct {
	X, Y, Width, Height float64
	RX, RY              float64 // 角丸半径
}

// Circle は円要素を表します
type Circle struct {
	CX, CY, R float64
}

// Ellipse は楕円要素を表します
type Ellipse struct {
	CX, CY, RX, RY float64
}

// Line は線要素を表します
type Line struct {
	X1, Y1, X2, Y2 float64
}

// Text はテキスト要素を表します
type Text struct {
	X, Y    float64
	Content string
}

// Point は2D点を表します
type Point struct {
	X, Y float64
}

// ============================================================
// ユーティリティ
// ============================================================

func maxF32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// parsePointsStrLocal は "x1,y1 x2,y2 ..." 形式のポイント文字列を解析します
func parsePointsStrLocal(s string) [][2]float64 {
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	var pts [][2]float64
	for i := 0; i+1 < len(parts); i += 2 {
		x, err1 := strconv.ParseFloat(parts[i], 64)
		y, err2 := strconv.ParseFloat(parts[i+1], 64)
		if err1 == nil && err2 == nil {
			pts = append(pts, [2]float64{x, y})
		}
	}
	return pts
}

// parseAttrF はElement属性を float64 として返します（デフォルト0）
func parseAttrF(elem *parser.Element, key string) float64 {
	v, ok := elem.Attributes[key]
	if !ok {
		return 0
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0
	}
	return f
}
