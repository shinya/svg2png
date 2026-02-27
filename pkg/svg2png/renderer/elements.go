package renderer

import (
	"log"
	"strconv"
	"strings"

	"github.com/shinya/svg2png/pkg/svg2png/parser"
	"github.com/shinya/svg2png/pkg/svg2png/raster"
	"github.com/shinya/svg2png/pkg/svg2png/style"
	"github.com/shinya/svg2png/pkg/svg2png/viewport"
)

// RenderElements はSVG要素を描画します
func RenderElements(doc *parser.Document, vp *viewport.Viewport, resolver *style.StyleResolver, rc *raster.RasterContext) error {
	return renderChildren(doc.Root.Children, vp, resolver, rc)
}

// renderChildren は子要素リストを描画します
func renderChildren(children []*parser.Element, vp *viewport.Viewport, resolver *style.StyleResolver, rc *raster.RasterContext) error {
	for _, child := range children {
		if err := renderElement(child, vp, resolver, rc); err != nil {
			return err
		}
	}
	return nil
}

// renderElement は個別の要素を描画します
func renderElement(elem *parser.Element, vp *viewport.Viewport, resolver *style.StyleResolver, rc *raster.RasterContext) error {
	log.Printf("Rendering element: <%s>", elem.Name)

	switch elem.Name {
	case "g", "svg":
		// グループ要素: 子要素を再帰的に描画
		st := resolver.Computed(elem)
		if st.ClipPathID != "" {
			rc.PushClipPath(st.ClipPathID)
			err := renderChildren(elem.Children, vp, resolver, rc)
			rc.PopClipPath()
			return err
		}
		return renderChildren(elem.Children, vp, resolver, rc)

	case "defs", "title", "desc", "metadata":
		// 描画しない要素
		return nil

	case "use":
		// TODO: use 要素のサポート
		return nil

	case "path":
		st := resolver.Computed(elem)
		return renderPath(elem, st, rc)

	case "rect":
		st := resolver.Computed(elem)
		return renderRect(elem, st, rc)

	case "circle":
		st := resolver.Computed(elem)
		return renderCircle(elem, st, rc)

	case "ellipse":
		st := resolver.Computed(elem)
		return renderEllipse(elem, st, rc)

	case "line":
		st := resolver.Computed(elem)
		return renderLine(elem, st, rc)

	case "polyline":
		st := resolver.Computed(elem)
		return renderPolyline(elem, st, rc, false)

	case "polygon":
		st := resolver.Computed(elem)
		return renderPolyline(elem, st, rc, true)

	case "text":
		st := resolver.Computed(elem)
		return renderText(elem, st, resolver, rc)

	case "tspan":
		// 単独で出現した場合はスキップ（通常は text 内から呼ばれる）
		return nil

	default:
		// 未対応の要素は子要素を描画
		return renderChildren(elem.Children, vp, resolver, rc)
	}
}

// renderPath はパス要素を描画します
func renderPath(elem *parser.Element, st *style.ComputedStyle, rc *raster.RasterContext) error {
	d := elem.Attributes["d"]
	if d == "" {
		return nil
	}
	path := &raster.Path{Data: d}
	rc.DrawPath(path, st)
	return nil
}

// renderRect は矩形要素を描画します
func renderRect(elem *parser.Element, st *style.ComputedStyle, rc *raster.RasterContext) error {
	rect := &raster.Rect{}

	if x, err := parseAttrFloat(elem, "x"); err == nil {
		rect.X = x
	}
	if y, err := parseAttrFloat(elem, "y"); err == nil {
		rect.Y = y
	}
	if w, err := parseAttrFloat(elem, "width"); err == nil {
		rect.Width = w
	}
	if h, err := parseAttrFloat(elem, "height"); err == nil {
		rect.Height = h
	}
	if rx, err := parseAttrFloat(elem, "rx"); err == nil {
		rect.RX = rx
	}
	if ry, err := parseAttrFloat(elem, "ry"); err == nil {
		rect.RY = ry
	}
	// rx/ry の相互フォールバック
	if rect.RX > 0 && rect.RY == 0 {
		rect.RY = rect.RX
	}
	if rect.RY > 0 && rect.RX == 0 {
		rect.RX = rect.RY
	}

	rc.DrawRect(rect, st)
	return nil
}

// renderCircle は円要素を描画します
func renderCircle(elem *parser.Element, st *style.ComputedStyle, rc *raster.RasterContext) error {
	circle := &raster.Circle{}

	if cx, err := parseAttrFloat(elem, "cx"); err == nil {
		circle.CX = cx
	}
	if cy, err := parseAttrFloat(elem, "cy"); err == nil {
		circle.CY = cy
	}
	if r, err := parseAttrFloat(elem, "r"); err == nil {
		circle.R = r
	}

	rc.DrawCircle(circle, st)
	return nil
}

// renderEllipse は楕円要素を描画します
func renderEllipse(elem *parser.Element, st *style.ComputedStyle, rc *raster.RasterContext) error {
	ellipse := &raster.Ellipse{}

	if cx, err := parseAttrFloat(elem, "cx"); err == nil {
		ellipse.CX = cx
	}
	if cy, err := parseAttrFloat(elem, "cy"); err == nil {
		ellipse.CY = cy
	}
	if rx, err := parseAttrFloat(elem, "rx"); err == nil {
		ellipse.RX = rx
	}
	if ry, err := parseAttrFloat(elem, "ry"); err == nil {
		ellipse.RY = ry
	}

	rc.DrawEllipse(ellipse, st)
	return nil
}

// renderLine は線要素を描画します
func renderLine(elem *parser.Element, st *style.ComputedStyle, rc *raster.RasterContext) error {
	line := &raster.Line{}

	if x1, err := parseAttrFloat(elem, "x1"); err == nil {
		line.X1 = x1
	}
	if y1, err := parseAttrFloat(elem, "y1"); err == nil {
		line.Y1 = y1
	}
	if x2, err := parseAttrFloat(elem, "x2"); err == nil {
		line.X2 = x2
	}
	if y2, err := parseAttrFloat(elem, "y2"); err == nil {
		line.Y2 = y2
	}

	rc.DrawLine(line, st)
	return nil
}

// renderPolyline はpolyline/polygon要素を描画します
func renderPolyline(elem *parser.Element, st *style.ComputedStyle, rc *raster.RasterContext, closed bool) error {
	pointsStr := elem.Attributes["points"]
	if pointsStr == "" {
		return nil
	}

	points := parsePoints(pointsStr)
	if len(points) == 0 {
		return nil
	}

	rc.DrawPolyline(points, st, closed)
	return nil
}

// renderText はテキスト要素を描画します
func renderText(elem *parser.Element, st *style.ComputedStyle, resolver *style.StyleResolver, rc *raster.RasterContext) error {
	// ベース位置を取得
	var baseX, baseY float64
	if x, err := parseAttrFloat(elem, "x"); err == nil {
		baseX = x
	}
	if y, err := parseAttrFloat(elem, "y"); err == nil {
		baseY = y
	}

	// tspan 子要素を持つかチェック
	var tspanChildren []*parser.Element
	for _, child := range elem.Children {
		if child.Name == "tspan" && child.Text != "" {
			// 絶対x位置を持たないtspan（フロー型）かチェック
			if _, err := parseAttrFloat(child, "x"); err != nil {
				tspanChildren = append(tspanChildren, child)
			}
		}
	}

	if len(tspanChildren) > 0 && elem.Text != "" {
		// フロー型テキスト: text + tspan を結合してグループ描画
		spans := make([]raster.TextSpan, 0)

		// 親のテキスト（末尾にスペースを追加）
		mainText := strings.TrimSpace(elem.Text)
		if mainText != "" {
			mainText += " "
			spans = append(spans, raster.TextSpan{Content: mainText, Style: st})
		}

		for _, child := range tspanChildren {
			childSt := resolver.ComputedFromParent(child, st)
			text := strings.TrimSpace(child.Text)
			if text != "" {
				spans = append(spans, raster.TextSpan{Content: text, Style: childSt})
			}
		}

		if len(spans) > 0 {
			rc.DrawTextGroup(spans, baseX, baseY, st.TextAnchor)
		}

		// 絶対位置を持つ tspan は別途描画
		for _, child := range elem.Children {
			if child.Name != "tspan" || child.Text == "" {
				continue
			}
			if _, err := parseAttrFloat(child, "x"); err == nil {
				// 絶対x位置を持つtspan
				childSt := resolver.ComputedFromParent(child, st)
				x, y := baseX, baseY
				if xv, err2 := parseAttrFloat(child, "x"); err2 == nil {
					x = xv
				}
				if yv, err2 := parseAttrFloat(child, "y"); err2 == nil {
					y = yv
				}
				if dx, err2 := parseAttrFloat(child, "dx"); err2 == nil {
					x += dx
				}
				if dy, err2 := parseAttrFloat(child, "dy"); err2 == nil {
					y += dy
				}
				text := &raster.Text{X: x, Y: y, Content: child.Text}
				rc.DrawText(text, childSt)
			}
		}
		return nil
	}

	// 単純な直接テキスト（tspan なし）
	if elem.Text != "" {
		text := &raster.Text{
			X:       baseX,
			Y:       baseY,
			Content: elem.Text,
		}
		rc.DrawText(text, st)
	}

	// tspan 子要素を個別に描画
	for _, child := range elem.Children {
		if child.Name != "tspan" {
			continue
		}

		// 親スタイルを継承して子のスタイルを計算
		childSt := resolver.ComputedFromParent(child, st)

		x := baseX
		y := baseY
		if xStr, err := parseAttrFloat(child, "x"); err == nil {
			x = xStr
		}
		if yStr, err := parseAttrFloat(child, "y"); err == nil {
			y = yStr
		}
		// dx/dy 属性（相対オフセット）
		if dx, err := parseAttrFloat(child, "dx"); err == nil {
			x += dx
		}
		if dy, err := parseAttrFloat(child, "dy"); err == nil {
			y += dy
		}

		content := child.Text
		if content != "" {
			text := &raster.Text{
				X:       x,
				Y:       y,
				Content: content,
			}
			rc.DrawText(text, childSt)
		}
	}

	return nil
}

// ============================================================
// ユーティリティ関数
// ============================================================

// parseAttrFloat は属性値をfloat64に変換します（単位を除去）
func parseAttrFloat(elem *parser.Element, attrName string) (float64, error) {
	v, ok := elem.Attributes[attrName]
	if !ok {
		return 0, strconv.ErrSyntax
	}
	v = strings.TrimSpace(v)
	// 単位を除去
	for _, suffix := range []string{"px", "pt", "em", "rem"} {
		if strings.HasSuffix(v, suffix) {
			v = strings.TrimSuffix(v, suffix)
			break
		}
	}
	return strconv.ParseFloat(strings.TrimSpace(v), 64)
}

// parsePoints はpoints属性を解析します（polyline/polygon用）
func parsePoints(pointsStr string) []raster.Point {
	// カンマとスペースで分割
	pointsStr = strings.ReplaceAll(pointsStr, ",", " ")
	parts := strings.Fields(pointsStr)

	var points []raster.Point
	for i := 0; i+1 < len(parts); i += 2 {
		x, err1 := strconv.ParseFloat(parts[i], 64)
		y, err2 := strconv.ParseFloat(parts[i+1], 64)
		if err1 == nil && err2 == nil {
			points = append(points, raster.Point{X: x, Y: y})
		}
	}
	return points
}
