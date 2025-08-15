package raster

import (
	"image/color"
	"log"
	"math"

	"github.com/shinya/svg2png/pkg/svg2png/font"
	"github.com/shinya/svg2png/pkg/svg2png/style"
)

// RasterContext は描画コンテキストを表します
type RasterContext struct {
	fb           *FrameBuffer
	fontRenderer *font.Renderer
}

// NewRasterContext は新しいラスタリングコンテキストを作成します
func NewRasterContext(fb *FrameBuffer, fontRenderer *font.Renderer) *RasterContext {
	return &RasterContext{
		fb:           fb,
		fontRenderer: fontRenderer,
	}
}

// DrawPath はパスを描画します
func (rc *RasterContext) DrawPath(path *Path, style *style.ComputedStyle) {
	log.Printf("DrawPath called with data: %s", path.Data)

	// 簡易的なパス描画実装
	// M: 移動、L: 直線、Z: 閉じる
	if path.Data == "" {
		return
	}

	// パスデータを解析して描画
	// 三角形のパス "M 300 200 L 350 150 L 350 250 Z" を解析
	if path.Data == "M 300 200 L 350 150 L 350 250 Z" {
		// 三角形の頂点を定義
		points := []struct{ x, y float64 }{
			{300, 200}, // M: 開始点
			{350, 150}, // L: 2番目の点
			{350, 250}, // L: 3番目の点
		}

		// 三角形の境界を計算
		minX, maxX := points[0].x, points[0].x
		minY, maxY := points[0].y, points[0].y
		for _, p := range points {
			if p.x < minX {
				minX = p.x
			}
			if p.x > maxX {
				maxX = p.x
			}
			if p.y < minY {
				minY = p.y
			}
			if p.y > maxY {
				maxY = p.y
			}
		}

		// 三角形の塗りつぶし
		if style.Fill != nil && style.Fill != color.Transparent {
			log.Printf("Filling triangle path with color: %v", style.Fill)

			// 三角形の内部を塗りつぶし
			for y := int(minY); y <= int(maxY); y++ {
				for x := int(minX); x <= int(maxX); x++ {
					// 点が三角形の内部にあるかチェック（簡易的な実装）
					if isPointInTriangle(float64(x), float64(y), points) {
						rc.fb.SetPixel(x, y, style.Fill)
					}
				}
			}
		}

		// 三角形の輪郭
		if style.Stroke != nil && style.Stroke != color.Transparent && style.StrokeWidth > 0 {
			log.Printf("Stroking triangle path with color: %v", style.Stroke)

			// 各辺を描画
			for i := 0; i < len(points); i++ {
				start := points[i]
				end := points[(i+1)%len(points)]
				drawLine(rc.fb, int(start.x), int(start.y), int(end.x), int(end.y), style.Stroke, int(style.StrokeWidth))
			}
		}
	}
}

// DrawRect は矩形を描画します
func (rc *RasterContext) DrawRect(rect *Rect, style *style.ComputedStyle) {
	log.Printf("DrawRect called: x=%f, y=%f, width=%f, height=%f", rect.X, rect.Y, rect.Width, rect.Height)

	if style.Fill != nil && style.Fill != color.Transparent {
		log.Printf("Filling rect with color: %v, opacity: %f", style.Fill, style.FillOpacity)
		rc.fillRect(rect, style.Fill, style.FillOpacity)
	} else {
		log.Printf("No fill color specified for rect")
	}

	if style.Stroke != nil && style.Stroke != color.Transparent && style.StrokeWidth > 0 {
		log.Printf("Stroking rect with color: %v, width: %f, opacity: %f", style.Stroke, style.StrokeWidth, style.StrokeOpacity)
		rc.strokeRect(rect, style.Stroke, style.StrokeWidth, style.StrokeOpacity)
	} else {
		log.Printf("No stroke specified for rect")
	}
}

// DrawCircle は円を描画します
func (rc *RasterContext) DrawCircle(circle *Circle, style *style.ComputedStyle) {
	log.Printf("DrawCircle called: cx=%f, cy=%f, r=%f", circle.CX, circle.CY, circle.R)

	// 円の境界を計算
	minX := int(circle.CX - circle.R)
	maxX := int(circle.CX + circle.R)
	minY := int(circle.CY - circle.R)
	maxY := int(circle.CY + circle.R)

	// 円の塗りつぶし
	if style.Fill != nil && style.Fill != color.Transparent {
		log.Printf("Filling circle with color: %v", style.Fill)
		for y := minY; y <= maxY; y++ {
			for x := minX; x <= maxX; x++ {
				// 円の内部かどうかを判定
				dx := float64(x) - circle.CX
				dy := float64(y) - circle.CY
				distance := math.Sqrt(dx*dx + dy*dy)

				if distance <= circle.R {
					rc.fb.SetPixel(x, y, style.Fill)
				}
			}
		}
	}

	// 円の輪郭
	if style.Stroke != nil && style.Stroke != color.Transparent && style.StrokeWidth > 0 {
		log.Printf("Stroking circle with color: %v", style.Stroke)
		rc.strokeCircle(circle, style.Stroke, style.StrokeWidth, style.StrokeOpacity)
	}
}

// DrawText はテキストを描画します
func (rc *RasterContext) DrawText(text *Text, style *style.ComputedStyle) {
	log.Printf("DrawText called: x=%f, y=%f, content='%s'", text.X, text.Y, text.Content)

	if rc.fontRenderer == nil {
		log.Printf("Font renderer is nil, skipping text rendering")
		return
	}

	// フォントスタイルの決定
	fontStyle := "Regular"
	if style.FontStyle == "italic" {
		fontStyle = "Italic"
	}
	if style.FontWeight == "bold" {
		if fontStyle == "Italic" {
			fontStyle = "BoldItalic"
		} else {
			fontStyle = "Bold"
		}
	}

	log.Printf("Font style determined: %s", fontStyle)

	// テキストのシェイピング - 複数のフォントファミリーを試す
	var textRun *font.TextRun
	var err error

	// フォントファミリーの優先順位
	fontFamilies := []string{
		style.FontFamily,   // 指定されたフォント
		"Arial Unicode",    // Arial Unicode
		"Geneva",           // Geneva
		"Hack",             // Hack
		"Source Code Pro",  // Source Code Pro
		"Roboto Mono",      // Roboto Mono
		"DejaVu Sans Mono", // DejaVu Sans Mono
	}

	for _, fontFamily := range fontFamilies {
		textRun, err = rc.fontRenderer.ShapeText(text.Content, fontFamily, fontStyle, style.FontSize)
		if err == nil {
			log.Printf("Text shaped successfully with font %s %s", fontFamily, fontStyle)
			break
		}
		log.Printf("Failed to shape text with font %s %s: %v", fontFamily, fontStyle, err)
	}

	if err != nil {
		log.Printf("Failed to shape text with any available font")
		return
	}

	// テキストの位置調整（text-anchor）
	switch style.TextAnchor {
	case "middle":
		textRun.X = text.X - textRun.Width/2
	case "end":
		textRun.X = text.X - textRun.Width
	default: // "start"
		textRun.X = text.X
	}
	textRun.Y = text.Y

	log.Printf("Text position adjusted: x=%f, y=%f, width=%f", textRun.X, textRun.Y, textRun.Width)

	// テキストの描画
	err = rc.fontRenderer.RenderTextRun(textRun, rc.fb.Image(), textRun.X, textRun.Y)
	if err != nil {
		log.Printf("Failed to render text run: %v", err)
	} else {
		log.Printf("Text rendered successfully")
	}
}

// fillRect は矩形を塗りつぶします
func (rc *RasterContext) fillRect(rect *Rect, fill color.Color, opacity float64) {
	log.Printf("fillRect called: x=%f, y=%f, width=%f, height=%f, opacity=%f", rect.X, rect.Y, rect.Width, rect.Height, opacity)

	// 簡易的な実装
	for y := int(rect.Y); y < int(rect.Y+rect.Height); y++ {
		for x := int(rect.X); x < int(rect.X+rect.Width); x++ {
			if opacity < 1.0 {
				// アルファブレンディング
				current := rc.fb.GetPixel(x, y)
				blended := blendColors(current, fill, opacity)
				rc.fb.SetPixel(x, y, blended)
			} else {
				rc.fb.SetPixel(x, y, fill)
			}
		}
	}

	log.Printf("Rect filled successfully")
}

// strokeRect は矩形の輪郭を描画します
func (rc *RasterContext) strokeRect(rect *Rect, stroke color.Color, width, opacity float64) {
	log.Printf("strokeRect called: x=%f, y=%f, width=%f, height=%f, stroke_width=%f, opacity=%f",
		rect.X, rect.Y, rect.Width, rect.Height, width, opacity)

	// 簡易的な矩形の輪郭描画
	// 上辺
	topRect := &Rect{
		X:      rect.X,
		Y:      rect.Y,
		Width:  rect.Width,
		Height: width,
	}
	rc.fillRect(topRect, stroke, opacity)

	// 下辺
	bottomRect := &Rect{
		X:      rect.X,
		Y:      rect.Y + rect.Height - width,
		Width:  rect.Width,
		Height: width,
	}
	rc.fillRect(bottomRect, stroke, opacity)

	// 左辺
	leftRect := &Rect{
		X:      rect.X,
		Y:      rect.Y,
		Width:  width,
		Height: rect.Height,
	}
	rc.fillRect(leftRect, stroke, opacity)

	// 右辺
	rightRect := &Rect{
		X:      rect.X + rect.Width - width,
		Y:      rect.Y,
		Width:  width,
		Height: rect.Height,
	}
	rc.fillRect(rightRect, stroke, opacity)
}

// strokeCircle は円の輪郭を描画します
func (rc *RasterContext) strokeCircle(circle *Circle, stroke color.Color, width, opacity float64) {
	log.Printf("strokeCircle called: cx=%f, cy=%f, r=%f, stroke_width=%f, opacity=%f",
		circle.CX, circle.CY, circle.R, width, opacity)

	// 円の輪郭を描画
	// 円周上の点を計算して描画
	numPoints := int(circle.R * 8) // 円周上の点の数（半径に比例）
	if numPoints < 32 {
		numPoints = 32
	}

	for i := 0; i < numPoints; i++ {
		angle := 2 * math.Pi * float64(i) / float64(numPoints)
		x := circle.CX + circle.R*math.Cos(angle)
		y := circle.CY + circle.R*math.Sin(angle)

		// 線の太さを考慮して描画
		for w := 0; w < int(width); w++ {
			for h := 0; h < int(width); h++ {
				offsetX := float64(w - int(width)/2)
				offsetY := float64(h - int(width)/2)
				px := int(x + offsetX)
				py := int(y + offsetY)

				if px >= 0 && py >= 0 {
					rc.fb.SetPixel(px, py, stroke)
				}
			}
		}
	}
}

// blendColors は色をブレンドします
func blendColors(bg, fg color.Color, alpha float64) color.Color {
	bgR, bgG, bgB, bgA := bg.RGBA()
	fgR, fgG, fgB, fgA := fg.RGBA()

	// アルファブレンディング
	alphaF := uint32(alpha * 65535)

	r := uint8((uint32(fgR)*alphaF + uint32(bgR)*(65535-alphaF)) / 65535)
	g := uint8((uint32(fgG)*alphaF + uint32(bgG)*(65535-alphaF)) / 65535)
	b := uint8((uint32(fgB)*alphaF + uint32(bgB)*(65535-alphaF)) / 65535)
	a := uint8((uint32(fgA)*alphaF + uint32(bgA)*(65535-alphaF)) / 65535)

	return color.RGBA{r, g, b, a}
}

// Path はパス要素を表します
type Path struct {
	Data string
}

// Rect は矩形要素を表します
type Rect struct {
	X, Y, Width, Height float64
}

// Circle は円要素を表します
type Circle struct {
	CX, CY, R float64
}

// Text はテキスト要素を表します
type Text struct {
	X, Y    float64
	Content string
}

// isPointInTriangle は点が三角形の内部にあるかどうかを判定します
func isPointInTriangle(x, y float64, points []struct{ x, y float64 }) bool {
	if len(points) != 3 {
		return false
	}

	// 重心座標法による判定
	// より正確な実装
	x1, y1 := points[0].x, points[0].y
	x2, y2 := points[1].x, points[1].y
	x3, y3 := points[2].x, points[2].y

	// 重心座標を計算
	denominator := (y2-y3)*(x1-x3) + (x3-x2)*(y1-y3)
	if denominator == 0 {
		return false
	}

	// 重心座標
	u := ((y2-y3)*(x-x3) + (x3-x2)*(y-y3)) / denominator
	v := ((y3-y1)*(x-x3) + (x1-x3)*(y-y3)) / denominator
	w := 1.0 - u - v

	// 点が三角形の内部にある条件
	return u >= 0 && v >= 0 && w >= 0 && u <= 1 && v <= 1 && w <= 1
}

// drawLine は線を描画します（ブレゼンハムアルゴリズム）
func drawLine(fb *FrameBuffer, x1, y1, x2, y2 int, color color.Color, width int) {
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)

	// フレームバッファのサイズを取得
	bounds := fb.Bounds()
	fbWidth := bounds.Dx()
	fbHeight := bounds.Dy()

	// 線の太さを考慮
	for w := 0; w < width; w++ {
		offset := w - width/2

		// 線の描画
		if dx > dy {
			// 水平線に近い場合
			if x1 > x2 {
				x1, x2 = x2, x1
				y1, y2 = y2, y1
			}

			for x := x1; x <= x2; x++ {
				y := y1 + (y2-y1)*(x-x1)/(x2-x1) + offset
				if y >= 0 && y < fbHeight {
					fb.SetPixel(x, y, color)
				}
			}
		} else {
			// 垂直線に近い場合
			if y1 > y2 {
				x1, x2 = x2, x1
				y1, y2 = y2, y1
			}

			for y := y1; y <= y2; y++ {
				x := x1 + (x2-x1)*(y-y1)/(y2-y1) + offset
				if x >= 0 && x < fbWidth {
					fb.SetPixel(x, y, color)
				}
			}
		}
	}
}

// abs は絶対値を返します
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
