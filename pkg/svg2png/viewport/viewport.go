package viewport

import (
	"fmt"
	"strconv"
	"strings"
	"github.com/shinya/svg2png/pkg/svg2png/parser"
)

// Viewport は解決されたビューポート情報を表します
type Viewport struct {
	Width  float64
	Height float64
	ViewBox *parser.ViewBox
	DPI    float64
}

// ResolveViewport はSVGドキュメントからビューポートを解決します
func ResolveViewport(doc *parser.Document, width, height int, dpi float64) (*Viewport, error) {
	vp := &Viewport{
		Width:   float64(width),
		Height:  float64(height),
		ViewBox: doc.ViewBox,
		DPI:     dpi,
	}
	
	// 幅と高さの解決
	if doc.Width != "" {
		if w, err := resolveDimension(doc.Width, vp.Width); err == nil {
			vp.Width = w
		}
	}
	
	if doc.Height != "" {
		if h, err := resolveDimension(doc.Height, vp.Height); err == nil {
			vp.Height = h
		}
	}
	
	// viewBoxがない場合は、幅と高さをそのまま使用
	if vp.ViewBox == nil {
		vp.ViewBox = &parser.ViewBox{
			X:      0,
			Y:      0,
			Width:  vp.Width,
			Height: vp.Height,
		}
	}
	
	return vp, nil
}

// resolveDimension は次元値を解決します（px、%、単位なし）
func resolveDimension(value string, reference float64) (float64, error) {
	value = strings.TrimSpace(value)
	
	// パーセント値の場合
	if strings.HasSuffix(value, "%") {
		percentStr := strings.TrimSuffix(value, "%")
		percent, err := strconv.ParseFloat(percentStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid percentage value: %s", value)
		}
		return reference * percent / 100.0, nil
	}
	
	// px単位の場合
	if strings.HasSuffix(value, "px") {
		pxStr := strings.TrimSuffix(value, "px")
		px, err := strconv.ParseFloat(pxStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid pixel value: %s", value)
		}
		return px, nil
	}
	
	// 単位なしの場合（pxとして扱う）
	if px, err := strconv.ParseFloat(value, 64); err == nil {
		return px, nil
	}
	
	return 0, fmt.Errorf("unsupported dimension format: %s", value)
}

// ConvertToPixels は座標値をピクセルに変換します
func (vp *Viewport) ConvertToPixels(x, y float64) (px, py float64) {
	if vp.ViewBox == nil {
		return x, y
	}
	
	// viewBoxからビューポートへの変換
	scaleX := vp.Width / vp.ViewBox.Width
	scaleY := vp.Height / vp.ViewBox.Height
	
	// パーセント値の場合
	if x < 0 || x > 1 {
		px = x
	} else {
		px = (x * vp.ViewBox.Width + vp.ViewBox.X) * scaleX
	}
	
	if y < 0 || y > 1 {
		py = y
	} else {
		py = (y * vp.ViewBox.Height + vp.ViewBox.Y) * scaleY
	}
	
	return px, py
}
