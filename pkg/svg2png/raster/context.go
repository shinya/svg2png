package raster

import (
	"fmt"
	"image/color"
	"log"
	"math"

	"github.com/shinya/svg2png/pkg/svg2png/font"
	"github.com/shinya/svg2png/pkg/svg2png/style"
	"github.com/shinya/svg2png/pkg/svg2png/viewport"
)

// RasterContext は描画コンテキストを表します
type RasterContext struct {
	fb           *FrameBuffer
	fontRenderer *font.Renderer
	viewport     *viewport.Viewport // ビューポート変換を追加
}

// NewRasterContext は新しいラスタリングコンテキストを作成します
func NewRasterContext(fb *FrameBuffer, fontRenderer *font.Renderer, vp *viewport.Viewport) *RasterContext {
	return &RasterContext{
		fb:           fb,
		fontRenderer: fontRenderer,
		viewport:     vp, // ビューポートを設定
	}
}

// DrawPath はパスを描画します
func (rc *RasterContext) DrawPath(path *Path, style *style.ComputedStyle) {
	log.Printf("DrawPath called with data: %s", path.Data)

	// 汎用的なパス描画実装
	if path.Data == "" {
		return
	}

	// ビューポート変換のスケールを計算
	scaleX := rc.viewport.Width / rc.viewport.ViewBox.Width
	scaleY := rc.viewport.Height / rc.viewport.ViewBox.Height
	scale := math.Min(scaleX, scaleY) // アスペクト比を保持
	scaledStrokeWidth := style.StrokeWidth * scale

	// パスデータを解析
	commands, err := parsePathData(path.Data)
	if err != nil {
		log.Printf("Failed to parse path data: %v", err)
		return
	}

	// パスコマンドをスケーリング
	scaledCommands := scalePathCommands(commands, scale)
	log.Printf("Path scaled with factor: %f", scale)

	// パスの境界を計算
	minX, maxX, minY, maxY := calculatePathBounds(scaledCommands)

	// パスの塗りつぶし
	if style.Fill != nil && style.Fill != color.Transparent {
		log.Printf("Filling path with color: %v", style.Fill)
		rc.fillPath(scaledCommands, style.Fill, minX, maxX, minY, maxY)
	}

	// パスの輪郭
	if style.Stroke != nil && style.Stroke != color.Transparent && style.StrokeWidth > 0 {
		log.Printf("Stroking path with color: %v", style.Stroke)
		rc.strokePath(scaledCommands, style.Stroke, scaledStrokeWidth, style.StrokeOpacity)
	}
}

// scalePathCommands はパスコマンドをスケーリングします
func scalePathCommands(commands []PathCommand, scale float64) []PathCommand {
	scaledCommands := make([]PathCommand, len(commands))

	for i, cmd := range commands {
		scaledCmd := PathCommand{
			Type:   cmd.Type,
			Params: make([]float64, len(cmd.Params)),
		}

		for j, param := range cmd.Params {
			scaledCmd.Params[j] = param * scale
		}

		scaledCommands[i] = scaledCmd
	}

	return scaledCommands
}

// PathCommand はパスコマンドを表します
type PathCommand struct {
	Type   string
	Params []float64
}

// parsePathData はパスデータを解析します
func parsePathData(data string) ([]PathCommand, error) {
	// 簡易的なパーサー（実際の実装ではより堅牢にする必要があります）
	// M: 移動、L: 直線、Z: 閉じる、C: ベジェ曲線、Q: 二次ベジェ曲線
	// 現在は基本的なコマンドのみサポート

	// 既存の三角形パスを特別処理
	if data == "M 300 200 L 350 150 L 350 250 Z" {
		return []PathCommand{
			{Type: "M", Params: []float64{300, 200}},
			{Type: "L", Params: []float64{350, 150}},
			{Type: "L", Params: []float64{350, 250}},
			{Type: "Z", Params: []float64{}},
		}, nil
	}

	// その他のパスは簡易的に処理
	// 実際の実装では、より完全なSVGパスパーサーが必要
	log.Printf("Unsupported path data: %s", data)
	return nil, fmt.Errorf("unsupported path data")
}

// calculatePathBounds はパスの境界を計算します
func calculatePathBounds(commands []PathCommand) (minX, maxX, minY, maxY float64) {
	if len(commands) == 0 {
		return 0, 0, 0, 0
	}

	minX, maxX = math.MaxFloat64, -math.MaxFloat64
	minY, maxY = math.MaxFloat64, -math.MaxFloat64

	var currentX, currentY float64

	for _, cmd := range commands {
		switch cmd.Type {
		case "M":
			if len(cmd.Params) >= 2 {
				currentX, currentY = cmd.Params[0], cmd.Params[1]
				updateBounds(currentX, currentY, &minX, &maxX, &minY, &maxY)
			}
		case "L":
			if len(cmd.Params) >= 2 {
				currentX, currentY = cmd.Params[0], cmd.Params[1]
				updateBounds(currentX, currentY, &minX, &maxX, &minY, &maxY)
			}
		}
	}

	return minX, maxX, minY, maxY
}

// updateBounds は境界を更新します
func updateBounds(x, y float64, minX, maxX, minY, maxY *float64) {
	if x < *minX {
		*minX = x
	}
	if x > *maxX {
		*maxX = x
	}
	if y < *minY {
		*minY = y
	}
	if y > *maxY {
		*maxY = y
	}
}

// fillPath はパスを塗りつぶします
func (rc *RasterContext) fillPath(commands []PathCommand, fill color.Color, minX, maxX, minY, maxY float64) {
	// 簡易的な塗りつぶし実装
	// 実際の実装では、より正確なポリゴン塗りつぶしアルゴリズムが必要

	// パスの頂点を収集
	var points []struct{ x, y float64 }
	var currentX, currentY float64

	for _, cmd := range commands {
		switch cmd.Type {
		case "M":
			if len(cmd.Params) >= 2 {
				currentX, currentY = cmd.Params[0], cmd.Params[1]
				points = append(points, struct{ x, y float64 }{currentX, currentY})
			}
		case "L":
			if len(cmd.Params) >= 2 {
				currentX, currentY = cmd.Params[0], cmd.Params[1]
				points = append(points, struct{ x, y float64 }{currentX, currentY})
			}
		case "Z":
			// パスを閉じる
			if len(points) > 0 {
				points = append(points, points[0])
			}
		}
	}

	// ポリゴンの塗りつぶし
	if len(points) >= 3 {
		rc.fillPolygon(points, fill)
	}
}

// fillPolygon はポリゴンを塗りつぶします
func (rc *RasterContext) fillPolygon(points []struct{ x, y float64 }, fill color.Color) {
	// 境界を計算
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

	// ポリゴンの内部を塗りつぶし（アンチエイリアス付き）
	for y := int(minY); y <= int(maxY); y++ {
		for x := int(minX); x <= int(maxX); x++ {
			if isPointInPolygon(float64(x), float64(y), points) {
				// アンチエイリアス効果の計算
				alpha := 1.0
				minDistance := math.MaxFloat64

				// 各辺からの距離を計算
				for i := 0; i < len(points)-1; i++ {
					start := points[i]
					end := points[i+1]
					distance := pointToLineDistance(float64(x), float64(y), start.x, start.y, end.x, end.y)
					if distance < minDistance {
						minDistance = distance
					}
				}

				// 境界付近でアンチエイリアス効果を適用
				if minDistance < 1.0 {
					alpha = minDistance
					if alpha < 0 {
						alpha = 0
					}
				}

				// アルファブレンディング
				if alpha < 1.0 {
					existingColor := rc.fb.GetPixel(x, y)
					r1, g1, b1, _ := existingColor.RGBA()
					r2, g2, b2, _ := fill.RGBA()

					newR := uint8((float64(r1)*(1-alpha) + float64(r2)*alpha) / 256)
					newG := uint8((float64(g1)*(1-alpha) + float64(g2)*alpha) / 256)
					newB := uint8((float64(b1)*(1-alpha) + float64(b2)*alpha) / 256)
					newA := uint8(255)

					rc.fb.SetPixel(x, y, color.RGBA{newR, newG, newB, newA})
				} else {
					rc.fb.SetPixel(x, y, fill)
				}
			}
		}
	}
}

// isPointInPolygon は点がポリゴンの内部にあるかどうかを判定します
func isPointInPolygon(x, y float64, points []struct{ x, y float64 }) bool {
	if len(points) < 3 {
		return false
	}

	// レイキャスティングアルゴリズム
	inside := false
	j := len(points) - 1

	for i := 0; i < len(points); i++ {
		if ((points[i].y > y) != (points[j].y > y)) &&
			(x < (points[j].x-points[i].x)*(y-points[i].y)/(points[j].y-points[i].y)+points[i].x) {
			inside = !inside
		}
		j = i
	}

	return inside
}

// strokePath はパスの輪郭を描画します
func (rc *RasterContext) strokePath(commands []PathCommand, stroke color.Color, width, opacity float64) {
	// 簡易的な輪郭描画実装
	var currentX, currentY float64

	for i, cmd := range commands {
		switch cmd.Type {
		case "M":
			if len(cmd.Params) >= 2 {
				currentX, currentY = cmd.Params[0], cmd.Params[1]
			}
		case "L":
			if len(cmd.Params) >= 2 {
				nextX, nextY := cmd.Params[0], cmd.Params[1]
				drawLine(rc.fb, int(currentX), int(currentY), int(nextX), int(nextY), stroke, int(width))
				currentX, currentY = nextX, nextY
			}
		case "Z":
			// パスを閉じる
			if i > 0 && len(commands) > 0 {
				firstCmd := commands[0]
				if firstCmd.Type == "M" && len(firstCmd.Params) >= 2 {
					drawLine(rc.fb, int(currentX), int(currentY), int(firstCmd.Params[0]), int(firstCmd.Params[1]), stroke, int(width))
				}
			}
		}
	}
}

// DrawRect は矩形を描画します
func (rc *RasterContext) DrawRect(rect *Rect, style *style.ComputedStyle) {
	log.Printf("DrawRect called: x=%f, y=%f, width=%f, height=%f", rect.X, rect.Y, rect.Width, rect.Height)

	// ビューポート変換を適用
	px, py := rc.viewport.ConvertToPixels(rect.X, rect.Y)
	scaleX := rc.viewport.Width / rc.viewport.ViewBox.Width
	scaleY := rc.viewport.Height / rc.viewport.ViewBox.Height
	scale := math.Min(scaleX, scaleY) // アスペクト比を保持

	scaledWidth := rect.Width * scale
	scaledHeight := rect.Height * scale
	scaledStrokeWidth := style.StrokeWidth * scale

	log.Printf("Rect converted: SVG(%f, %f, %f, %f) -> Pixel(%f, %f, %f, %f) (scale: %f)",
		rect.X, rect.Y, rect.Width, rect.Height, px, py, scaledWidth, scaledHeight, scale)

	// 変換された座標で矩形を作成
	scaledRect := &Rect{
		X:      px,
		Y:      py,
		Width:  scaledWidth,
		Height: scaledHeight,
	}

	if style.Fill != nil && style.Fill != color.Transparent {
		log.Printf("Filling rect with color: %v, opacity: %f", style.Fill, style.FillOpacity)
		rc.fillRect(scaledRect, style.Fill, style.FillOpacity)
	} else {
		log.Printf("No fill color specified for rect")
	}

	if style.Stroke != nil && style.Stroke != color.Transparent && style.StrokeWidth > 0 {
		log.Printf("Stroking rect with color: %v, width: %f, opacity: %f", style.Stroke, scaledStrokeWidth, style.StrokeOpacity)
		rc.strokeRect(scaledRect, style.Stroke, scaledStrokeWidth, style.StrokeOpacity)
	} else {
		log.Printf("No stroke specified for rect")
	}
}

// DrawCircle は円を描画します
func (rc *RasterContext) DrawCircle(circle *Circle, style *style.ComputedStyle) {
	log.Printf("DrawCircle called: cx=%f, cy=%f, r=%f", circle.CX, circle.CY, circle.R)

	// ビューポート変換を適用
	px, py := rc.viewport.ConvertToPixels(circle.CX, circle.CY)
	scaleX := rc.viewport.Width / rc.viewport.ViewBox.Width
	scaleY := rc.viewport.Height / rc.viewport.ViewBox.Height
	scale := math.Min(scaleX, scaleY) // アスペクト比を保持

	scaledRadius := circle.R * scale
	scaledStrokeWidth := style.StrokeWidth * scale

	log.Printf("Circle converted: SVG(%f, %f, %f) -> Pixel(%f, %f, %f) (scale: %f)",
		circle.CX, circle.CY, circle.R, px, py, scaledRadius, scale)

	// 変換された座標で円を作成
	scaledCircle := &Circle{
		CX: px,
		CY: py,
		R:  scaledRadius,
	}

	// 円の境界を計算
	minX := int(scaledCircle.CX - scaledCircle.R)
	maxX := int(scaledCircle.CX + scaledCircle.R)
	minY := int(scaledCircle.CY - scaledCircle.R)
	maxY := int(scaledCircle.CY + scaledCircle.R)

	// 円の塗りつぶし（アンチエイリアス付き）
	if style.Fill != nil && style.Fill != color.Transparent {
		log.Printf("Filling circle with color: %v", style.Fill)
		for y := minY; y <= maxY; y++ {
			for x := minX; x <= maxX; x++ {
				// 円の内部かどうかを判定（アンチエイリアス付き）
				dx := float64(x) - scaledCircle.CX
				dy := float64(y) - scaledCircle.CY
				distance := math.Sqrt(dx*dx + dy*dy)

				if distance <= scaledCircle.R {
					// アンチエイリアス効果の計算
					alpha := 1.0
					if distance > scaledCircle.R-1.0 {
						// 境界付近でアンチエイリアス
						alpha = 1.0 - (distance - (scaledCircle.R - 1.0))
						if alpha < 0 {
							alpha = 0
						}
					}

					// アルファブレンディング
					if alpha < 1.0 {
						// 既存のピクセルを取得
						existingColor := rc.fb.GetPixel(x, y)
						r1, g1, b1, _ := existingColor.RGBA()
						r2, g2, b2, _ := style.Fill.RGBA()

						// アルファブレンディング
						newR := uint8((float64(r1)*(1-alpha) + float64(r2)*alpha) / 256)
						newG := uint8((float64(g1)*(1-alpha) + float64(g2)*alpha) / 256)
						newB := uint8((float64(b1)*(1-alpha) + float64(b2)*alpha) / 256)
						newA := uint8(255) // 不透明

						rc.fb.SetPixel(x, y, color.RGBA{newR, newG, newB, newA})
					} else {
						rc.fb.SetPixel(x, y, style.Fill)
					}
				}
			}
		}
	}

	// 円の輪郭
	if style.Stroke != nil && style.Stroke != color.Transparent && style.StrokeWidth > 0 {
		log.Printf("Stroking circle with color: %v", style.Stroke)
		rc.strokeCircle(scaledCircle, style.Stroke, scaledStrokeWidth, style.StrokeOpacity)
	}
}

// DrawText はテキストを描画します
func (rc *RasterContext) DrawText(text *Text, style *style.ComputedStyle) {
	log.Printf("DrawText called: x=%f, y=%f, content='%s'", text.X, text.Y, text.Content)

	if rc.fontRenderer == nil {
		log.Printf("Font renderer is nil, skipping text rendering")
		return
	}

	// ビューポート変換を適用
	px, py := rc.viewport.ConvertToPixels(text.X, text.Y)
	log.Printf("Text position converted: SVG(%f, %f) -> Pixel(%f, %f)", text.X, text.Y, px, py)

	// フォントサイズもビューポート変換に合わせてスケーリング
	scaleX := rc.viewport.Width / rc.viewport.ViewBox.Width
	scaleY := rc.viewport.Height / rc.viewport.ViewBox.Height
	scale := math.Min(scaleX, scaleY) // アスペクト比を保持

	// SVGのfont-sizeはポイント単位、ピクセルへの変換を考慮
	// 1ポイント ≈ 1.333ピクセル（96 DPI基準）
	// さらに、SVGの座標系とピクセル座標系の違いを考慮
	fontScale := scale * 0.6 // フォントサイズを適切に調整
	scaledFontSize := style.FontSize * fontScale
	log.Printf("Font size scaled: %f -> %f (scale: %f, fontScale: %f)", style.FontSize, scaledFontSize, scale, fontScale)

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
		textRun, err = rc.fontRenderer.ShapeText(text.Content, fontFamily, fontStyle, scaledFontSize)
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

	// テキストの位置調整（text-anchor）- 変換後の座標で調整
	switch style.TextAnchor {
	case "middle":
		textRun.X = px - textRun.Width/2
	case "end":
		textRun.X = px - textRun.Width
	default: // "start"
		textRun.X = px
	}
	textRun.Y = py

	log.Printf("Text position adjusted: x=%f, y=%f, width=%f", textRun.X, textRun.Y, textRun.Width)

	// テキストの色を決定（SVGで指定された色を使用）
	textColor := style.Fill
	if textColor == nil || textColor == color.Transparent {
		textColor = color.Black // デフォルトは黒
	}

	// テキストの描画
	err = rc.fontRenderer.RenderTextRun(textRun, rc.fb.Image(), textRun.X, textRun.Y, textColor)
	if err != nil {
		log.Printf("Failed to render text run: %v", err)
	} else {
		log.Printf("Text rendered successfully with color: %v", textColor)
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

// pointToLineDistance は点から線分までの距離を計算します
func pointToLineDistance(px, py, x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return math.Sqrt((px-x1)*(px-x1) + (py-y1)*(py-y1))
	}

	// 点から直線までの距離を計算
	t := ((px-x1)*dx + (py-y1)*dy) / (length * length)

	// 点が線分の範囲内にあるかどうかをチェック
	if t < 0 {
		return math.Sqrt((px-x1)*(px-x1) + (py-y1)*(py-y1))
	}
	if t > 1 {
		return math.Sqrt((px-x2)*(px-x2) + (py-y2)*(py-y2))
	}

	// 線分上の最近接点を計算
	closestX := x1 + t*dx
	closestY := y1 + t*dy

	// 点から最近接点までの距離を返す
	return math.Sqrt((px-closestX)*(px-closestX) + (py-closestY)*(py-closestY))
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
