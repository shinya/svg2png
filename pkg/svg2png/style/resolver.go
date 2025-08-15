package style

import (
	"image/color"
	"strconv"
	"strings"
	"github.com/shinya/svg2png/pkg/svg2png/parser"
)

// ComputedStyle は計算されたスタイルを表します
type ComputedStyle struct {
	Fill         color.Color
	FillOpacity  float64
	Stroke       color.Color
	StrokeWidth  float64
	StrokeOpacity float64
	Opacity      float64
	FontFamily   string
	FontSize     float64
	FontStyle    string
	FontWeight   string
	TextAnchor   string
}

// StyleResolver はスタイルの解決を行います
type StyleResolver struct {
	defaultFamily string
	diagnostics   *Diagnostics
}

// Diagnostics は診断情報を表します
type Diagnostics struct {
	Warnings    []string
	MissingFonts []string
	Unsupported []string
}

// NewResolver は新しいスタイル解決器を作成します
func NewResolver(defaultFamily string) *StyleResolver {
	return &StyleResolver{
		defaultFamily: defaultFamily,
		diagnostics:   &Diagnostics{},
	}
}

// Computed は要素の計算されたスタイルを返します
func (r *StyleResolver) Computed(elem *parser.Element) *ComputedStyle {
	style := &ComputedStyle{
		Fill:         color.Black,
		FillOpacity:  1.0,
		Stroke:       color.Transparent,
		StrokeWidth:  0,
		StrokeOpacity: 1.0,
		Opacity:      1.0,
		FontFamily:   r.defaultFamily,
		FontSize:     12,
		FontStyle:    "normal",
		FontWeight:   "normal",
		TextAnchor:   "start",
	}
	
	// プレゼンテーション属性の適用
	r.applyPresentationAttributes(elem, style)
	
	// style属性の適用
	r.applyStyleAttribute(elem, style)
	
	// 継承の適用
	r.applyInheritance(elem, style)
	
	return style
}

// applyPresentationAttributes はプレゼンテーション属性を適用します
func (r *StyleResolver) applyPresentationAttributes(elem *parser.Element, style *ComputedStyle) {
	for key, value := range elem.Attributes {
		switch key {
		case "fill":
			if value != "none" {
				if c, err := parseColor(value); err == nil {
					style.Fill = c
				}
			}
		case "fill-opacity":
			if opacity, err := strconv.ParseFloat(value, 64); err == nil {
				style.FillOpacity = opacity
			}
		case "stroke":
			if value != "none" {
				if c, err := parseColor(value); err == nil {
					style.Stroke = c
				}
			}
		case "stroke-width":
			if width, err := strconv.ParseFloat(value, 64); err == nil {
				style.StrokeWidth = width
			}
		case "stroke-opacity":
			if opacity, err := strconv.ParseFloat(value, 64); err == nil {
				style.StrokeOpacity = opacity
			}
		case "opacity":
			if opacity, err := strconv.ParseFloat(value, 64); err == nil {
				style.Opacity = opacity
			}
		case "font-family":
			style.FontFamily = value
		case "font-size":
			if size, err := strconv.ParseFloat(value, 64); err == nil {
				style.FontSize = size
			}
		case "font-style":
			style.FontStyle = value
		case "font-weight":
			style.FontWeight = value
		case "text-anchor":
			style.TextAnchor = value
		}
	}
}

// applyStyleAttribute はstyle属性を適用します
func (r *StyleResolver) applyStyleAttribute(elem *parser.Element, style *ComputedStyle) {
	if styleAttr, exists := elem.Attributes["style"]; exists {
		// 簡易的なstyle属性の解析
		pairs := strings.Split(styleAttr, ";")
		for _, pair := range pairs {
			kv := strings.SplitN(pair, ":", 2)
			if len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				value := strings.TrimSpace(kv[1])
				
				switch key {
				case "fill":
					if value != "none" {
						if c, err := parseColor(value); err == nil {
							style.Fill = c
						}
					}
				case "stroke":
					if value != "none" {
						if c, err := parseColor(value); err == nil {
							style.Stroke = c
						}
					}
				// 他のスタイルプロパティも同様に処理
				}
			}
		}
	}
}

// applyInheritance は継承を適用します
func (r *StyleResolver) applyInheritance(elem *parser.Element, style *ComputedStyle) {
	// 現在の実装では、親要素からの継承は簡易的に処理
	// 実際の実装では、要素ツリーを辿って継承を適用する必要がある
}

// parseColor は色値を解析します
func parseColor(value string) (color.Color, error) {
	value = strings.TrimSpace(value)
	
	// 名前付き色
	if c, ok := namedColors[value]; ok {
		return c, nil
	}
	
	// 16進数色
	if strings.HasPrefix(value, "#") {
		return parseHexColor(value)
	}
	
	// RGB色
	if strings.HasPrefix(value, "rgb(") {
		return parseRGBColor(value)
	}
	
	// currentColor
	if value == "currentColor" {
		return color.Black, nil // デフォルト値
	}
	
	return color.Black, nil
}

// parseHexColor は16進数色を解析します
func parseHexColor(hex string) (color.Color, error) {
	hex = strings.TrimPrefix(hex, "#")
	
	var r, g, b uint8
	
	switch len(hex) {
	case 3:
		// #RGB
		if rv, err := strconv.ParseUint(string(hex[0])+string(hex[0]), 16, 8); err == nil {
			r = uint8(rv)
		}
		if gv, err := strconv.ParseUint(string(hex[1])+string(hex[1]), 16, 8); err == nil {
			g = uint8(gv)
		}
		if bv, err := strconv.ParseUint(string(hex[2])+string(hex[2]), 16, 8); err == nil {
			b = uint8(bv)
		}
	case 6:
		// #RRGGBB
		if rv, err := strconv.ParseUint(hex[0:2], 16, 8); err == nil {
			r = uint8(rv)
		}
		if gv, err := strconv.ParseUint(hex[2:4], 16, 8); err == nil {
			g = uint8(gv)
		}
		if bv, err := strconv.ParseUint(hex[4:6], 16, 8); err == nil {
			b = uint8(bv)
		}
	default:
		return color.Black, nil
	}
	
	return color.RGBA{r, g, b, 255}, nil
}

// parseRGBColor はRGB色を解析します
func parseRGBColor(rgb string) (color.Color, error) {
	// 簡易的な実装
	// 実際の実装では、より詳細な解析が必要
	return color.Black, nil
}

// namedColors は名前付き色のマップです
var namedColors = map[string]color.Color{
	"black":   color.Black,
	"white":   color.White,
	"red":     color.RGBA{255, 0, 0, 255},
	"green":   color.RGBA{0, 255, 0, 255},
	"blue":    color.RGBA{0, 0, 255, 255},
	"transparent": color.Transparent,
}

// GetDiagnostics は診断情報を返します
func (r *StyleResolver) GetDiagnostics() Diagnostics {
	return *r.diagnostics
}
