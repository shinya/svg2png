package raster

import (
	"image"
	"image/color"
	"image/png"
	"bytes"
)

// FrameBuffer は画像の描画バッファを表します
type FrameBuffer struct {
	img        *image.RGBA
	background *color.RGBA
}

// NewFrameBuffer は新しいフレームバッファを作成します
func NewFrameBuffer(width, height int, background *color.RGBA) *FrameBuffer {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// 背景色の設定
	if background != nil {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				img.Set(x, y, *background)
			}
		}
	}
	
	return &FrameBuffer{
		img:        img,
		background: background,
	}
}

// SetPixel は指定された座標にピクセルを設定します
func (fb *FrameBuffer) SetPixel(x, y int, c color.Color) {
	if x >= 0 && x < fb.img.Bounds().Dx() && y >= 0 && y < fb.img.Bounds().Dy() {
		fb.img.Set(x, y, c)
	}
}

// GetPixel は指定された座標のピクセルを取得します
func (fb *FrameBuffer) GetPixel(x, y int) color.Color {
	if x >= 0 && x < fb.img.Bounds().Dx() && y >= 0 && y < fb.img.Bounds().Dy() {
		return fb.img.At(x, y)
	}
	return color.Transparent
}

// Bounds はフレームバッファの境界を返します
func (fb *FrameBuffer) Bounds() image.Rectangle {
	return fb.img.Bounds()
}

// EncodePNG はフレームバッファをPNG形式でエンコードします
func (fb *FrameBuffer) EncodePNG() ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, fb.img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Image は内部の画像を返します
func (fb *FrameBuffer) Image() *image.RGBA {
	return fb.img
}
