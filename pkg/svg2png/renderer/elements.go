package renderer

import (
	"log"
	"strconv"

	"github.com/shinya/svg2png/pkg/svg2png/parser"
	"github.com/shinya/svg2png/pkg/svg2png/raster"
	"github.com/shinya/svg2png/pkg/svg2png/style"
	"github.com/shinya/svg2png/pkg/svg2png/viewport"
)

// RenderElements はSVG要素を描画します
func RenderElements(doc *parser.Document, vp *viewport.Viewport, resolver *style.StyleResolver, rc *raster.RasterContext) error {
	log.Printf("Starting to render elements, document has %d root children", len(doc.Root.Children))
	return renderElement(doc.Root, vp, resolver, rc)
}

// renderElement は個別の要素を描画します
func renderElement(elem *parser.Element, vp *viewport.Viewport, resolver *style.StyleResolver, rc *raster.RasterContext) error {
	log.Printf("Rendering element: %s with %d children", elem.Name, len(elem.Children))

	// スタイルの解決
	style := resolver.Computed(elem)
	log.Printf("Style computed for %s: font-family=%s, font-size=%f", elem.Name, style.FontFamily, style.FontSize)

	// 要素の種類に応じた描画
	switch elem.Name {
	case "path":
		log.Printf("Rendering path element")
		return renderPath(elem, style, rc)
	case "rect":
		log.Printf("Rendering rect element")
		return renderRect(elem, style, rc)
	case "circle":
		log.Printf("Rendering circle element")
		return renderCircle(elem, style, rc)
	case "text":
		log.Printf("Rendering text element: '%s'", elem.Text)
		return renderText(elem, style, rc)
	}

	// 子要素の再帰描画
	for _, child := range elem.Children {
		if err := renderElement(child, vp, resolver, rc); err != nil {
			return err
		}
	}

	return nil
}

// renderPath はパス要素を描画します
func renderPath(elem *parser.Element, style *style.ComputedStyle, rc *raster.RasterContext) error {
	// 簡易的な実装
	// 実際の実装では、SVGパスの詳細な解析が必要
	path := &raster.Path{
		Data: elem.Attributes["d"],
	}

	log.Printf("Path data: %s", path.Data)
	rc.DrawPath(path, style)
	return nil
}

// renderRect は矩形要素を描画します
func renderRect(elem *parser.Element, style *style.ComputedStyle, rc *raster.RasterContext) error {
	rect := &raster.Rect{}

	// 座標とサイズの解析
	if x, err := strconv.ParseFloat(elem.Attributes["x"], 64); err == nil {
		rect.X = x
	}
	if y, err := strconv.ParseFloat(elem.Attributes["y"], 64); err == nil {
		rect.Y = y
	}
	if width, err := strconv.ParseFloat(elem.Attributes["width"], 64); err == nil {
		rect.Width = width
	}
	if height, err := strconv.ParseFloat(elem.Attributes["height"], 64); err == nil {
		rect.Height = height
	}

	log.Printf("Rect: x=%f, y=%f, width=%f, height=%f", rect.X, rect.Y, rect.Width, rect.Height)
	rc.DrawRect(rect, style)
	return nil
}

// renderCircle は円要素を描画します
func renderCircle(elem *parser.Element, style *style.ComputedStyle, rc *raster.RasterContext) error {
	circle := &raster.Circle{}

	// 中心座標と半径の解析
	if cx, err := strconv.ParseFloat(elem.Attributes["cx"], 64); err == nil {
		circle.CX = cx
	}
	if cy, err := strconv.ParseFloat(elem.Attributes["cy"], 64); err == nil {
		circle.CY = cy
	}
	if r, err := strconv.ParseFloat(elem.Attributes["r"], 64); err == nil {
		circle.R = r
	}

	log.Printf("Circle: cx=%f, cy=%f, r=%f", circle.CX, circle.CY, circle.R)
	rc.DrawCircle(circle, style)
	return nil
}

// renderText はテキスト要素を描画します
func renderText(elem *parser.Element, style *style.ComputedStyle, rc *raster.RasterContext) error {
	text := &raster.Text{}

	// 座標の解析
	if x, err := strconv.ParseFloat(elem.Attributes["x"], 64); err == nil {
		text.X = x
	}
	if y, err := strconv.ParseFloat(elem.Attributes["y"], 64); err == nil {
		text.Y = y
	}

	// テキストコンテンツの取得
	text.Content = elem.Text

	log.Printf("Text: x=%f, y=%f, content='%s'", text.X, text.Y, text.Content)

	// 簡易的なテキスト描画
	// 実際の実装では、フォントレンダリングが必要
	rc.DrawText(text, style)
	return nil
}
