package raster

import (
	"image"
	"image/color"
	"math"

	"github.com/shinya/svg2png/pkg/svg2png/parser"
)

// applyFilterToLayer はフィルター定義を画像レイヤーに適用し、結果をメイン fb に合成します
func (rc *RasterContext) applyFilterToLayer(layer *image.RGBA, filterID string, opacity float64) {
	if rc.defs == nil {
		rc.compositeRGBALayer(layer, opacity)
		return
	}
	fd, ok := rc.defs.Filters[filterID]
	if !ok {
		rc.compositeRGBALayer(layer, opacity)
		return
	}

	// 名前付き中間レイヤーのマップ
	namedLayers := map[string]*image.RGBA{
		"SourceGraphic": layer,
	}

	current := layer

	// SVGユーザー座標→ピクセル座標のスケール
	scaleX, scaleY, _, _ := rc.scales()

	for _, prim := range fd.Primitives {
		switch prim.Type {
		case "feGaussianBlur":
			inLayer := resolveFilterInput(prim.In, current, namedLayers, layer)
			// stdDeviation は SVG ユーザー単位 → ピクセル単位に変換
			sigmaX := prim.StdDeviationX * scaleX
			sigmaY := prim.StdDeviationY * scaleY
			blurred := GaussianBlurRGBA(inLayer, sigmaX, sigmaY)
			if prim.Result != "" {
				namedLayers[prim.Result] = blurred
			}
			current = blurred

		case "feComposite":
			if prim.Operator == "over" || prim.Operator == "" {
				// in を in2 の上に重ねる（SVG仕様通り）
				inLayer := resolveFilterInput(prim.In, current, namedLayers, layer)
				in2Layer := resolveFilterInput(prim.In2, current, namedLayers, layer)
				result := compositeOver(inLayer, in2Layer, layer.Bounds())
				if prim.Result != "" {
					namedLayers[prim.Result] = result
				}
				current = result
			}
		}
	}

	rc.compositeRGBALayer(current, opacity)
}

// resolveFilterInput はフィルタープリミティブの入力を解決します
func resolveFilterInput(in string, current *image.RGBA, namedLayers map[string]*image.RGBA, sourceGraphic *image.RGBA) *image.RGBA {
	if in == "" {
		return current
	}
	if in == "SourceGraphic" {
		return sourceGraphic
	}
	if layer, ok := namedLayers[in]; ok {
		return layer
	}
	return current
}

// GaussianBlurRGBA はRGBA画像にガウシアンブラーを適用した新しい画像を返します
// プリマルチプライドアルファ空間でブラーを行い、ストローク付近の色情報を保持します
func GaussianBlurRGBA(src *image.RGBA, sigmaX, sigmaY float64) *image.RGBA {
	if sigmaX <= 0 && sigmaY <= 0 {
		return src
	}
	if sigmaX <= 0 {
		sigmaX = sigmaY
	}
	if sigmaY <= 0 {
		sigmaY = sigmaX
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// ストレート→プリマルチプライドアルファに変換
	pm := toPreMultiplied(src, w, h, bounds)

	kernelX := gaussianKernel(sigmaX)
	kernelY := gaussianKernel(sigmaY)

	// 水平パス（プリマルチプライドアルファ空間）
	tmp := image.NewRGBA(bounds)
	applyKernelH(pm, tmp, kernelX, w, h)

	// 垂直パス
	pm2 := image.NewRGBA(bounds)
	applyKernelV(tmp, pm2, kernelY, w, h)

	// プリマルチプライド→ストレートアルファに逆変換
	return fromPreMultiplied(pm2, w, h, bounds)
}

// toPreMultiplied はストレートアルファ RGBA をプリマルチプライドアルファに変換します
func toPreMultiplied(src *image.RGBA, w, h int, bounds image.Rectangle) *image.RGBA {
	pm := image.NewRGBA(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.RGBAAt(x, y)
			a := float64(c.A) / 255.0
			pm.SetRGBA(x, y, color.RGBA{
				R: uint8(float64(c.R) * a),
				G: uint8(float64(c.G) * a),
				B: uint8(float64(c.B) * a),
				A: c.A,
			})
		}
	}
	return pm
}

// fromPreMultiplied はプリマルチプライドアルファをストレートアルファに逆変換します
func fromPreMultiplied(pm *image.RGBA, w, h int, bounds image.Rectangle) *image.RGBA {
	dst := image.NewRGBA(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := pm.RGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			a := float64(c.A) / 255.0
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(clampFloat(float64(c.R) / a)),
				G: uint8(clampFloat(float64(c.G) / a)),
				B: uint8(clampFloat(float64(c.B) / a)),
				A: c.A,
			})
		}
	}
	return dst
}

// gaussianKernel は正規化されたガウシアンカーネルを返します（半径 = ceil(3σ)）
func gaussianKernel(sigma float64) []float64 {
	radius := int(math.Ceil(sigma * 3))
	if radius < 1 {
		radius = 1
	}
	size := 2*radius + 1
	kernel := make([]float64, size)
	sum := 0.0
	for i := range kernel {
		x := float64(i - radius)
		kernel[i] = math.Exp(-0.5 * x * x / (sigma * sigma))
		sum += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	return kernel
}

// applyKernelH は水平方向の畳み込みを行います
func applyKernelH(src, dst *image.RGBA, kernel []float64, w, h int) {
	radius := len(kernel) / 2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b, a float64
			for ki, kv := range kernel {
				xi := x + ki - radius
				if xi < 0 {
					xi = 0
				} else if xi >= w {
					xi = w - 1
				}
				c := src.RGBAAt(xi, y)
				r += float64(c.R) * kv
				g += float64(c.G) * kv
				b += float64(c.B) * kv
				a += float64(c.A) * kv
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(clampFloat(r)),
				G: uint8(clampFloat(g)),
				B: uint8(clampFloat(b)),
				A: uint8(clampFloat(a)),
			})
		}
	}
}

// applyKernelV は垂直方向の畳み込みを行います
func applyKernelV(src, dst *image.RGBA, kernel []float64, w, h int) {
	radius := len(kernel) / 2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b, a float64
			for ki, kv := range kernel {
				yi := y + ki - radius
				if yi < 0 {
					yi = 0
				} else if yi >= h {
					yi = h - 1
				}
				c := src.RGBAAt(x, yi)
				r += float64(c.R) * kv
				g += float64(c.G) * kv
				b += float64(c.B) * kv
				a += float64(c.A) * kv
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(clampFloat(r)),
				G: uint8(clampFloat(g)),
				B: uint8(clampFloat(b)),
				A: uint8(clampFloat(a)),
			})
		}
	}
}

// compositeOver は over 画像を under 画像の上に Porter-Duff over 合成します
func compositeOver(over, under *image.RGBA, bounds image.Rectangle) *image.RGBA {
	dst := image.NewRGBA(bounds)
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			fg := over.RGBAAt(px, py)
			bg := under.RGBAAt(px, py)

			if fg.A == 0 {
				dst.SetRGBA(px, py, bg)
				continue
			}
			if bg.A == 0 {
				dst.SetRGBA(px, py, fg)
				continue
			}

			fa := float64(fg.A) / 255.0
			ba := float64(bg.A) / 255.0
			outA := fa + ba*(1-fa)

			newR := uint8((float64(fg.R)*fa + float64(bg.R)*ba*(1-fa)) / outA)
			newG := uint8((float64(fg.G)*fa + float64(bg.G)*ba*(1-fa)) / outA)
			newB := uint8((float64(fg.B)*fa + float64(bg.B)*ba*(1-fa)) / outA)
			newA := uint8(clampFloat(outA * 255))

			dst.SetRGBA(px, py, color.RGBA{newR, newG, newB, newA})
		}
	}
	return dst
}

// compositeRGBALayer はRGBAレイヤーをフレームバッファにアルファブレンドします
func (rc *RasterContext) compositeRGBALayer(layer *image.RGBA, opacity float64) {
	img := rc.fb.Image()
	bounds := img.Bounds()
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			src := layer.RGBAAt(px, py)
			if src.A == 0 {
				continue
			}
			a := float64(src.A) / 255.0 * opacity
			if a <= 0 {
				continue
			}
			bg := img.RGBAAt(px, py)
			newR := uint8(float64(src.R)*a + float64(bg.R)*(1-a))
			newG := uint8(float64(src.G)*a + float64(bg.G)*(1-a))
			newB := uint8(float64(src.B)*a + float64(bg.B)*(1-a))
			newA := uint8(clampFloat(float64(bg.A) + float64(src.A)*opacity*(1-float64(bg.A)/255.0)))
			img.SetRGBA(px, py, color.RGBA{newR, newG, newB, newA})
		}
	}
}

// renderToTempBuffer は同じ設定の一時的な RasterContext を作成します（フィルター適用用）
func (rc *RasterContext) renderToTempBuffer() *RasterContext {
	w, h := rc.fb.Bounds().Dx(), rc.fb.Bounds().Dy()
	tempFB := NewFrameBuffer(w, h, nil)
	return &RasterContext{
		fb:           tempFB,
		fontRenderer: rc.fontRenderer,
		viewport:     rc.viewport,
		defs:         rc.defs,
		clipMask:     rc.clipMask,
		// filterID は設定しない（再帰防止）
	}
}

// applyFilterDef はフィルター定義を取得します（defs から）
func (rc *RasterContext) getFilterDef(filterID string) *parser.FilterDef {
	if rc.defs == nil {
		return nil
	}
	fd, ok := rc.defs.Filters[filterID]
	if !ok {
		return nil
	}
	return fd
}

// clampFloat は float64 値を [0, 255] にクランプします
func clampFloat(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
