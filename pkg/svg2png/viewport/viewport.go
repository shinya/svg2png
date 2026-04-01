package viewport

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shinya/svg2png/pkg/svg2png/parser"
)

// Viewport は解決されたビューポート情報を表します
type Viewport struct {
	Width   float64
	Height  float64
	ViewBox *parser.ViewBox
	DPI     float64
	// uniform scaling（preserveAspectRatio対応）
	Scale   float64 // 均一スケール値
	OffsetX float64 // X方向オフセット（ピクセル単位、センタリング用）
	OffsetY float64 // Y方向オフセット（ピクセル単位、センタリング用）
}

// ResolveViewport はSVGドキュメントからビューポートを解決します
// scaleFactor はWidth/Heightが0の場合に適用されるスケール倍率（0以下の場合は1.0扱い）
func ResolveViewport(doc *parser.Document, width, height int, dpi float64, scaleFactor ...float64) (*Viewport, error) {
	sf := 1.0
	if len(scaleFactor) > 0 && scaleFactor[0] > 0 {
		sf = scaleFactor[0]
	}
	vp := &Viewport{
		Width:  float64(width),
		Height: float64(height),
		DPI:    dpi,
	}

	// viewBoxの解決
	if doc.ViewBox != nil {
		vp.ViewBox = doc.ViewBox
	} else {
		// viewBoxがない場合、SVGのwidth/height属性から推定
		vbW := vp.Width
		vbH := vp.Height
		if doc.Width != "" {
			if w, err := resolveDimension(doc.Width, vp.Width); err == nil {
				vbW = w
			}
		}
		if doc.Height != "" {
			if h, err := resolveDimension(doc.Height, vp.Height); err == nil {
				vbH = h
			}
		}
		vp.ViewBox = &parser.ViewBox{
			X: 0, Y: 0,
			Width:  vbW,
			Height: vbH,
		}
	}

	// width/heightが0の場合、SVGのviewBoxまたはwidth/height属性から自動計算
	if vp.Width == 0 && vp.Height == 0 {
		vp.Width = vp.ViewBox.Width
		vp.Height = vp.ViewBox.Height
		if doc.Width != "" {
			if w, err := resolveDimension(doc.Width, 0); err == nil && w > 0 {
				vp.Width = w
			}
		}
		if doc.Height != "" {
			if h, err := resolveDimension(doc.Height, 0); err == nil && h > 0 {
				vp.Height = h
			}
		}
		// スケール倍率を適用
		vp.Width *= sf
		vp.Height *= sf
	} else if vp.Width == 0 {
		// heightのみ指定 → widthをアスペクト比から計算
		vp.Width = vp.Height * vp.ViewBox.Width / vp.ViewBox.Height
	} else if vp.Height == 0 {
		// widthのみ指定 → heightをアスペクト比から計算
		vp.Height = vp.Width * vp.ViewBox.Height / vp.ViewBox.Width
	}

	// preserveAspectRatio: xMidYMid meet（SVGデフォルト）
	// 均一スケーリングでセンタリング
	scaleX := vp.Width / vp.ViewBox.Width
	scaleY := vp.Height / vp.ViewBox.Height
	vp.Scale = scaleX
	if scaleY < scaleX {
		vp.Scale = scaleY
	}
	// meet: コンテンツ全体が見えるように小さい方のスケールを使用
	// xMidYMid: 水平・垂直方向の中央寄せ
	vp.OffsetX = (vp.Width - vp.ViewBox.Width*vp.Scale) / 2
	vp.OffsetY = (vp.Height - vp.ViewBox.Height*vp.Scale) / 2

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

	// SVG座標系から出力画像座標系への変換
	px = (x - vp.ViewBox.X) * scaleX
	py = (y - vp.ViewBox.Y) * scaleY

	return px, py
}
