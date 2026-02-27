package raster

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/shinya/svg2png/pkg/svg2png/parser"
	"github.com/shinya/svg2png/pkg/svg2png/style"
)

// gradStop はレンダリング用のグラデーションストップです
type gradStop struct {
	offset float64
	col    color.RGBA
}

// resolveStops はparser.GradientStopスライスをレンダリング用に変換します
func resolveStops(stops []parser.GradientStop) []gradStop {
	var result []gradStop
	for _, s := range stops {
		c := color.RGBA{0, 0, 0, 255}
		if s.Color != "" {
			if parsed, err := style.ParseColor(s.Color); err == nil {
				r, g, b, a := parsed.RGBA()
				c = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
			}
		}
		// stop-opacity を A チャネルに掛ける
		c.A = uint8(float64(c.A) * clampGrad(s.Opacity))
		result = append(result, gradStop{offset: s.Offset, col: c})
	}
	return result
}

func clampGrad(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// interpolateStops はt(0-1)に対応する補間色を返します
func interpolateStops(stops []gradStop, t float64) color.RGBA {
	if len(stops) == 0 {
		return color.RGBA{0, 0, 0, 255}
	}
	if t <= stops[0].offset {
		return stops[0].col
	}
	if t >= stops[len(stops)-1].offset {
		return stops[len(stops)-1].col
	}
	for i := 1; i < len(stops); i++ {
		if t <= stops[i].offset {
			s0, s1 := stops[i-1], stops[i]
			d := s1.offset - s0.offset
			if d <= 0 {
				return s1.col
			}
			f := (t - s0.offset) / d
			return color.RGBA{
				R: uint8(float64(s0.col.R)*(1-f) + float64(s1.col.R)*f),
				G: uint8(float64(s0.col.G)*(1-f) + float64(s1.col.G)*f),
				B: uint8(float64(s0.col.B)*(1-f) + float64(s1.col.B)*f),
				A: uint8(float64(s0.col.A)*(1-f) + float64(s1.col.A)*f),
			}
		}
	}
	return stops[len(stops)-1].col
}

// parseGradCoordRatio は "50%" や "0.5" を 0-1 比率として返します
func parseGradCoordRatio(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if strings.HasSuffix(s, "%") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		return v / 100.0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseGradCoordAbs は "50%" を total に対する絶対値、または数値をそのまま返します
func parseGradCoordAbs(s string, total float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if strings.HasSuffix(s, "%") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		return v / 100.0 * total
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// compositeGradPixel はグラデーション色をアルファ値で合成します
func compositeGradPixel(img *image.RGBA, px, py int, col color.RGBA, maskA uint8, opacity float64) {
	if maskA == 0 {
		return
	}
	// col.A × mask × opacity で最終アルファを決める
	a := float64(maskA) / 255.0 * opacity * float64(col.A) / 255.0
	if a <= 0 {
		return
	}
	bg := img.RGBAAt(px, py)
	newR := uint8(float64(col.R)*a + float64(bg.R)*(1-a))
	newG := uint8(float64(col.G)*a + float64(bg.G)*(1-a))
	newB := uint8(float64(col.B)*a + float64(bg.B)*(1-a))
	newA := uint8(math.Min(255, float64(bg.A)+255*a))
	img.SetRGBA(px, py, color.RGBA{newR, newG, newB, newA})
}

// DrawLinearGradient は線形グラデーションをalphaマスク領域に描画します
// bounds: 描画対象のピクセル境界（グラデーション座標の基準）
func (rc *RasterContext) DrawLinearGradient(
	alpha *image.Alpha,
	lg *parser.LinearGradient,
	bounds image.Rectangle,
	opacity float64,
) {
	stops := resolveStops(lg.Stops)
	if len(stops) == 0 {
		return
	}

	img := rc.fb.Image()
	bw := float64(bounds.Dx())
	bh := float64(bounds.Dy())
	bx := float64(bounds.Min.X)
	by := float64(bounds.Min.Y)

	gradUnits := lg.GradientUnits
	if gradUnits == "" {
		gradUnits = "objectBoundingBox"
	}

	var gx1, gy1, gx2, gy2 float64

	if gradUnits == "userSpaceOnUse" {
		scaleX, scaleY, offX, offY := rc.scales()
		x1Raw, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(lg.X1), "%"), 64)
		y1Raw, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(lg.Y1), "%"), 64)
		x2Raw, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(lg.X2), "%"), 64)
		y2Raw, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(lg.Y2), "%"), 64)
		// % の場合はviewBox幅高さに対する割合
		vbW := rc.viewport.ViewBox.Width
		vbH := rc.viewport.ViewBox.Height
		if strings.HasSuffix(strings.TrimSpace(lg.X1), "%") {
			x1Raw = x1Raw / 100 * vbW
		}
		if strings.HasSuffix(strings.TrimSpace(lg.Y1), "%") {
			y1Raw = y1Raw / 100 * vbH
		}
		if strings.HasSuffix(strings.TrimSpace(lg.X2), "%") {
			x2Raw = x2Raw / 100 * vbW
		}
		if strings.HasSuffix(strings.TrimSpace(lg.Y2), "%") {
			y2Raw = y2Raw / 100 * vbH
		}
		gx1 = x1Raw*scaleX + offX
		gy1 = y1Raw*scaleY + offY
		gx2 = x2Raw*scaleX + offX
		gy2 = y2Raw*scaleY + offY
	} else {
		// objectBoundingBox
		gx1 = bx + parseGradCoordRatio(lg.X1)*bw
		gy1 = by + parseGradCoordRatio(lg.Y1)*bh
		gx2 = bx + parseGradCoordRatio(lg.X2)*bw
		gy2 = by + parseGradCoordRatio(lg.Y2)*bh
	}

	dx := gx2 - gx1
	dy := gy2 - gy1
	lenSq := dx*dx + dy*dy

	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			maskA := alpha.AlphaAt(px, py).A
			if maskA == 0 {
				continue
			}
			var t float64
			if lenSq > 0 {
				t = ((float64(px)-gx1)*dx + (float64(py)-gy1)*dy) / lenSq
			}
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			col := interpolateStops(stops, t)
			compositeGradPixel(img, px, py, col, maskA, opacity)
		}
	}
}

// DrawRadialGradient は放射状グラデーションをalphaマスク領域に描画します
func (rc *RasterContext) DrawRadialGradient(
	alpha *image.Alpha,
	rg *parser.RadialGradient,
	bounds image.Rectangle,
	opacity float64,
) {
	stops := resolveStops(rg.Stops)
	if len(stops) == 0 {
		return
	}

	img := rc.fb.Image()
	bw := float64(bounds.Dx())
	bh := float64(bounds.Dy())
	bx := float64(bounds.Min.X)
	by := float64(bounds.Min.Y)

	gradUnits := rg.GradientUnits
	if gradUnits == "" {
		gradUnits = "objectBoundingBox"
	}

	var cx, cy, r float64

	if gradUnits == "userSpaceOnUse" {
		scaleX, scaleY, offX, offY := rc.scales()
		vbW := rc.viewport.ViewBox.Width
		vbH := rc.viewport.ViewBox.Height
		cxRaw := parseGradCoordAbs(rg.CX, vbW)
		cyRaw := parseGradCoordAbs(rg.CY, vbH)
		rRaw := parseGradCoordAbs(rg.R, math.Sqrt(vbW*vbW+vbH*vbH)/math.Sqrt2)
		cx = cxRaw*scaleX + offX
		cy = cyRaw*scaleY + offY
		r = rRaw * (scaleX + scaleY) / 2
	} else {
		// objectBoundingBox
		cx = bx + parseGradCoordRatio(rg.CX)*bw
		cy = by + parseGradCoordRatio(rg.CY)*bh
		// r はバウンディングボックスの短辺の半分を基準
		rRatio := parseGradCoordRatio(rg.R)
		// 楕円的に対応: x方向とy方向それぞれにrを掛ける（楕円radial）
		rx := rRatio * bw
		ry := rRatio * bh

		for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
			for px := bounds.Min.X; px < bounds.Max.X; px++ {
				maskA := alpha.AlphaAt(px, py).A
				if maskA == 0 {
					continue
				}
				var t float64
				if rx > 0 && ry > 0 {
					ddx := (float64(px) - cx) / rx
					ddy := (float64(py) - cy) / ry
					t = math.Sqrt(ddx*ddx + ddy*ddy)
				}
				if t < 0 {
					t = 0
				}
				if t > 1 {
					t = 1
				}
				col := interpolateStops(stops, t)
				compositeGradPixel(img, px, py, col, maskA, opacity)
			}
		}
		return
	}

	// userSpaceOnUse の場合
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			maskA := alpha.AlphaAt(px, py).A
			if maskA == 0 {
				continue
			}
			var t float64
			if r > 0 {
				dx := float64(px) - cx
				dy := float64(py) - cy
				t = math.Sqrt(dx*dx+dy*dy) / r
			}
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			col := interpolateStops(stops, t)
			compositeGradPixel(img, px, py, col, maskA, opacity)
		}
	}
}

// DrawPatternFill はパターンでアルファマスク領域を塗りつぶします（簡易実装）
func (rc *RasterContext) DrawPatternFill(
	alpha *image.Alpha,
	patternElem *parser.Element,
	bounds image.Rectangle,
	opacity float64,
	defs *parser.Defs,
) {
	// パターンのサイズを取得
	pwStr := patternElem.Attributes["width"]
	phStr := patternElem.Attributes["height"]
	if pwStr == "" || phStr == "" {
		return
	}
	pw, err1 := strconv.ParseFloat(pwStr, 64)
	ph, err2 := strconv.ParseFloat(phStr, 64)
	if err1 != nil || err2 != nil || pw <= 0 || ph <= 0 {
		return
	}

	scaleX, scaleY, _, _ := rc.scales()
	scaledPW := pw * scaleX
	scaledPH := ph * scaleY

	// パターンのタイル画像を作成
	tileBounds := image.Rect(0, 0, int(math.Ceil(scaledPW)), int(math.Ceil(scaledPH)))
	if tileBounds.Dx() == 0 || tileBounds.Dy() == 0 {
		return
	}

	// 簡易的なパターンレンダリング: パターン内の図形を描画
	tile := image.NewRGBA(tileBounds)

	// パターン要素の子要素を一時的なコンテキストで描画（簡易版）
	for _, child := range patternElem.Children {
		if child.Name == "circle" {
			cxStr := child.Attributes["cx"]
			cyStr := child.Attributes["cy"]
			rStr := child.Attributes["r"]
			cx, _ := strconv.ParseFloat(cxStr, 64)
			cy, _ := strconv.ParseFloat(cyStr, 64)
			r, _ := strconv.ParseFloat(rStr, 64)
			fillStr := child.Attributes["fill"]
			fillOpStr := child.Attributes["fill-opacity"]

			col := color.RGBA{0, 0, 0, 255}
			if parsed, err := style.ParseColor(fillStr); err == nil {
				rv, gv, bv, av := parsed.RGBA()
				col = color.RGBA{uint8(rv >> 8), uint8(gv >> 8), uint8(bv >> 8), uint8(av >> 8)}
			}
			fillOp := 1.0
			if v, err := strconv.ParseFloat(fillOpStr, 64); err == nil {
				fillOp = v
			}

			// タイル内相対座標（オフセットは加算しない）
			pcx := cx * scaleX
			pcy := cy * scaleY
			pr := r * (scaleX + scaleY) / 2

			// 小円を描画
			for iy := 0; iy < tileBounds.Dy(); iy++ {
				for ix := 0; ix < tileBounds.Dx(); ix++ {
					dx := float64(ix) - pcx
					dy := float64(iy) - pcy
					if dx*dx+dy*dy <= pr*pr {
						a := fillOp * float64(col.A) / 255.0
						bg := tile.RGBAAt(ix, iy)
						tile.SetRGBA(ix, iy, color.RGBA{
							R: uint8(float64(col.R)*a + float64(bg.R)*(1-a)),
							G: uint8(float64(col.G)*a + float64(bg.G)*(1-a)),
							B: uint8(float64(col.B)*a + float64(bg.B)*(1-a)),
							A: uint8(math.Min(255, float64(bg.A)+255*a)),
						})
					}
				}
			}
		}
	}

	// タイルをターゲットに繰り返す
	img := rc.fb.Image()
	tileW := tileBounds.Dx()
	tileH := tileBounds.Dy()
	if tileW == 0 || tileH == 0 {
		return
	}

	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			maskA := alpha.AlphaAt(px, py).A
			if maskA == 0 {
				continue
			}
			// タイル座標
			tx := ((px - bounds.Min.X) % tileW + tileW) % tileW
			ty := ((py - bounds.Min.Y) % tileH + tileH) % tileH
			tcol := tile.RGBAAt(tx, ty)
			if tcol.A == 0 {
				continue
			}
			a := float64(maskA) / 255.0 * opacity * float64(tcol.A) / 255.0
			bg := img.RGBAAt(px, py)
			img.SetRGBA(px, py, color.RGBA{
				R: uint8(float64(tcol.R)*a + float64(bg.R)*(1-a)),
				G: uint8(float64(tcol.G)*a + float64(bg.G)*(1-a)),
				B: uint8(float64(tcol.B)*a + float64(bg.B)*(1-a)),
				A: uint8(math.Min(255, float64(bg.A)+255*a)),
			})
		}
	}
}
