package style

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/shinya/svg2png/pkg/svg2png/parser"
)

// ComputedStyle は計算されたスタイルを表します
type ComputedStyle struct {
	Fill          color.Color
	FillNone      bool // fill="none" が明示的に指定された
	FillOpacity   float64
	Stroke        color.Color
	StrokeNone    bool // stroke="none" が明示的に指定された
	StrokeWidth   float64
	StrokeOpacity float64
	Opacity       float64
	FontFamily    string
	FontSize      float64
	FontStyle     string
	FontWeight    string
	TextAnchor    string
}

// StyleResolver はスタイルの解決を行います
type StyleResolver struct {
	defaultFamily string
	diagnostics   *Diagnostics
}

// Diagnostics は診断情報を表します
type Diagnostics struct {
	Warnings     []string
	MissingFonts []string
	Unsupported  []string
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
		Fill:          color.Black,
		FillOpacity:   1.0,
		Stroke:        color.Transparent,
		StrokeNone:    true, // デフォルトは stroke なし
		StrokeWidth:   1.0,
		StrokeOpacity: 1.0,
		Opacity:       1.0,
		FontFamily:    r.defaultFamily,
		FontSize:      12,
		FontStyle:     "normal",
		FontWeight:    "normal",
		TextAnchor:    "start",
	}

	// プレゼンテーション属性の適用（style属性より優先度低）
	r.applyPresentationAttributes(elem, style)

	// style属性の適用（最優先）
	r.applyStyleAttribute(elem, style)

	return style
}

// applyPresentationAttributes はプレゼンテーション属性を適用します
func (r *StyleResolver) applyPresentationAttributes(elem *parser.Element, style *ComputedStyle) {
	for key, value := range elem.Attributes {
		r.applyProperty(key, strings.TrimSpace(value), style)
	}
}

// applyStyleAttribute はstyle属性を適用します
func (r *StyleResolver) applyStyleAttribute(elem *parser.Element, style *ComputedStyle) {
	styleAttr, exists := elem.Attributes["style"]
	if !exists {
		return
	}

	pairs := strings.Split(styleAttr, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		r.applyProperty(key, value, style)
	}
}

// applyProperty は単一のCSSプロパティを適用します
func (r *StyleResolver) applyProperty(key, value string, style *ComputedStyle) {
	switch key {
	case "fill":
		if value == "none" {
			style.Fill = color.Transparent
			style.FillNone = true
		} else {
			if c, err := parseColor(value); err == nil {
				style.Fill = c
				style.FillNone = false
			}
		}
	case "fill-opacity":
		if opacity, err := strconv.ParseFloat(value, 64); err == nil {
			style.FillOpacity = clamp01(opacity)
		}
	case "stroke":
		if value == "none" {
			style.Stroke = color.Transparent
			style.StrokeNone = true
		} else {
			if c, err := parseColor(value); err == nil {
				style.Stroke = c
				style.StrokeNone = false
			}
		}
	case "stroke-width":
		if width, err := parseDimension(value); err == nil {
			style.StrokeWidth = width
		}
	case "stroke-opacity":
		if opacity, err := strconv.ParseFloat(value, 64); err == nil {
			style.StrokeOpacity = clamp01(opacity)
		}
	case "opacity":
		if opacity, err := strconv.ParseFloat(value, 64); err == nil {
			style.Opacity = clamp01(opacity)
		}
	case "font-family":
		style.FontFamily = strings.Trim(value, `'"`)
	case "font-size":
		if size, err := parseFontSize(value); err == nil {
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

// parseFontSize はフォントサイズを解析します（単位付きも対応）
func parseFontSize(value string) (float64, error) {
	value = strings.TrimSpace(value)
	// 単位を除去
	for _, suffix := range []string{"px", "pt", "em", "rem", "ex", "ch", "vw", "vh"} {
		if strings.HasSuffix(value, suffix) {
			numStr := strings.TrimSuffix(value, suffix)
			v, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
			if err != nil {
				return 0, err
			}
			// pt は px に変換（1pt = 1.333px @ 96dpi）
			if suffix == "pt" {
				v = v * 96.0 / 72.0
			}
			return v, nil
		}
	}
	// 単位なし（px として扱う）
	return strconv.ParseFloat(value, 64)
}

// parseDimension は次元値を解析します
func parseDimension(value string) (float64, error) {
	value = strings.TrimSpace(value)
	for _, suffix := range []string{"px", "pt", "em", "rem"} {
		if strings.HasSuffix(value, suffix) {
			return strconv.ParseFloat(strings.TrimSuffix(value, suffix), 64)
		}
	}
	return strconv.ParseFloat(value, 64)
}

// clamp01 は値を [0, 1] の範囲に制限します
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// parseColor は色値を解析します
func parseColor(value string) (color.Color, error) {
	value = strings.TrimSpace(value)

	if value == "none" || value == "transparent" {
		return color.Transparent, nil
	}

	// currentColor はデフォルトで黒
	if value == "currentColor" {
		return color.Black, nil
	}

	// 名前付き色
	if c, ok := namedColors[strings.ToLower(value)]; ok {
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

	// RGBA色
	if strings.HasPrefix(value, "rgba(") {
		return parseRGBAColor(value)
	}

	return color.Black, nil
}

// parseHexColor は16進数色を解析します
func parseHexColor(hex string) (color.Color, error) {
	hex = strings.TrimPrefix(hex, "#")

	switch len(hex) {
	case 3:
		// #RGB → #RRGGBB
		r, _ := strconv.ParseUint(string(hex[0])+string(hex[0]), 16, 8)
		g, _ := strconv.ParseUint(string(hex[1])+string(hex[1]), 16, 8)
		b, _ := strconv.ParseUint(string(hex[2])+string(hex[2]), 16, 8)
		return color.RGBA{uint8(r), uint8(g), uint8(b), 255}, nil
	case 4:
		// #RGBA
		r, _ := strconv.ParseUint(string(hex[0])+string(hex[0]), 16, 8)
		g, _ := strconv.ParseUint(string(hex[1])+string(hex[1]), 16, 8)
		b, _ := strconv.ParseUint(string(hex[2])+string(hex[2]), 16, 8)
		a, _ := strconv.ParseUint(string(hex[3])+string(hex[3]), 16, 8)
		return color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}, nil
	case 6:
		// #RRGGBB
		r, _ := strconv.ParseUint(hex[0:2], 16, 8)
		g, _ := strconv.ParseUint(hex[2:4], 16, 8)
		b, _ := strconv.ParseUint(hex[4:6], 16, 8)
		return color.RGBA{uint8(r), uint8(g), uint8(b), 255}, nil
	case 8:
		// #RRGGBBAA
		r, _ := strconv.ParseUint(hex[0:2], 16, 8)
		g, _ := strconv.ParseUint(hex[2:4], 16, 8)
		b, _ := strconv.ParseUint(hex[4:6], 16, 8)
		a, _ := strconv.ParseUint(hex[6:8], 16, 8)
		return color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}, nil
	default:
		return color.Black, nil
	}
}

// parseRGBColor はRGB色を解析します
func parseRGBColor(rgb string) (color.Color, error) {
	// rgb(r, g, b) または rgb(r% g% b%)
	rgb = strings.TrimPrefix(rgb, "rgb(")
	rgb = strings.TrimSuffix(rgb, ")")
	rgb = strings.ReplaceAll(rgb, ",", " ")
	parts := strings.Fields(rgb)
	if len(parts) != 3 {
		return color.Black, nil
	}

	parseComponent := func(s string) uint8 {
		s = strings.TrimSpace(s)
		if strings.HasSuffix(s, "%") {
			v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
			if err != nil {
				return 0
			}
			return uint8(clamp01(v/100) * 255)
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		return uint8(v)
	}

	return color.RGBA{
		R: parseComponent(parts[0]),
		G: parseComponent(parts[1]),
		B: parseComponent(parts[2]),
		A: 255,
	}, nil
}

// parseRGBAColor はRGBA色を解析します
func parseRGBAColor(rgba string) (color.Color, error) {
	rgba = strings.TrimPrefix(rgba, "rgba(")
	rgba = strings.TrimSuffix(rgba, ")")
	rgba = strings.ReplaceAll(rgba, ",", " ")
	parts := strings.Fields(rgba)
	if len(parts) != 4 {
		return color.Black, nil
	}

	c, err := parseRGBColor("rgb(" + strings.Join(parts[:3], " ") + ")")
	if err != nil {
		return color.Black, nil
	}
	rgba2 := c.(color.RGBA)

	alpha, _ := strconv.ParseFloat(parts[3], 64)
	rgba2.A = uint8(clamp01(alpha) * 255)
	return rgba2, nil
}

// ComputedFromParent は親スタイルを継承した上で要素のスタイルを計算します
// tspan などの子要素で、親の値を引き継ぎつつ上書きする場合に使用します
func (r *StyleResolver) ComputedFromParent(elem *parser.Element, parent *ComputedStyle) *ComputedStyle {
	st := *parent // 親のスタイルをコピー
	r.applyPresentationAttributes(elem, &st)
	r.applyStyleAttribute(elem, &st)
	return &st
}

// GetDiagnostics は診断情報を返します
func (r *StyleResolver) GetDiagnostics() Diagnostics {
	return *r.diagnostics
}

// namedColors は名前付き色のマップです（CSS Color Level 4 準拠）
var namedColors = map[string]color.Color{
	"black":                color.RGBA{0, 0, 0, 255},
	"white":                color.RGBA{255, 255, 255, 255},
	"red":                  color.RGBA{255, 0, 0, 255},
	"lime":                 color.RGBA{0, 255, 0, 255},
	"green":                color.RGBA{0, 128, 0, 255},
	"blue":                 color.RGBA{0, 0, 255, 255},
	"yellow":               color.RGBA{255, 255, 0, 255},
	"cyan":                 color.RGBA{0, 255, 255, 255},
	"aqua":                 color.RGBA{0, 255, 255, 255},
	"magenta":              color.RGBA{255, 0, 255, 255},
	"fuchsia":              color.RGBA{255, 0, 255, 255},
	"gray":                 color.RGBA{128, 128, 128, 255},
	"grey":                 color.RGBA{128, 128, 128, 255},
	"silver":               color.RGBA{192, 192, 192, 255},
	"maroon":               color.RGBA{128, 0, 0, 255},
	"olive":                color.RGBA{128, 128, 0, 255},
	"navy":                 color.RGBA{0, 0, 128, 255},
	"teal":                 color.RGBA{0, 128, 128, 255},
	"purple":               color.RGBA{128, 0, 128, 255},
	"orange":               color.RGBA{255, 165, 0, 255},
	"orangered":            color.RGBA{255, 69, 0, 255},
	"pink":                 color.RGBA{255, 192, 203, 255},
	"hotpink":              color.RGBA{255, 105, 180, 255},
	"deeppink":             color.RGBA{255, 20, 147, 255},
	"brown":                color.RGBA{165, 42, 42, 255},
	"saddlebrown":          color.RGBA{139, 69, 19, 255},
	"darkred":              color.RGBA{139, 0, 0, 255},
	"darkgreen":            color.RGBA{0, 100, 0, 255},
	"darkblue":             color.RGBA{0, 0, 139, 255},
	"darkgray":             color.RGBA{169, 169, 169, 255},
	"darkgrey":             color.RGBA{169, 169, 169, 255},
	"lightgray":            color.RGBA{211, 211, 211, 255},
	"lightgrey":            color.RGBA{211, 211, 211, 255},
	"dimgray":              color.RGBA{105, 105, 105, 255},
	"dimgrey":              color.RGBA{105, 105, 105, 255},
	"lightblue":            color.RGBA{173, 216, 230, 255},
	"lightyellow":          color.RGBA{255, 255, 224, 255},
	"lightgreen":           color.RGBA{144, 238, 144, 255},
	"lightcoral":           color.RGBA{240, 128, 128, 255},
	"lightsalmon":          color.RGBA{255, 160, 122, 255},
	"lightseagreen":        color.RGBA{32, 178, 170, 255},
	"lightskyblue":         color.RGBA{135, 206, 250, 255},
	"lightslategray":       color.RGBA{119, 136, 153, 255},
	"lightsteelblue":       color.RGBA{176, 196, 222, 255},
	"cornflowerblue":       color.RGBA{100, 149, 237, 255},
	"royalblue":            color.RGBA{65, 105, 225, 255},
	"mediumblue":           color.RGBA{0, 0, 205, 255},
	"dodgerblue":           color.RGBA{30, 144, 255, 255},
	"steelblue":            color.RGBA{70, 130, 180, 255},
	"deepskyblue":          color.RGBA{0, 191, 255, 255},
	"skyblue":              color.RGBA{135, 206, 235, 255},
	"cadetblue":            color.RGBA{95, 158, 160, 255},
	"powderblue":           color.RGBA{176, 224, 230, 255},
	"aliceblue":            color.RGBA{240, 248, 255, 255},
	"slateblue":            color.RGBA{106, 90, 205, 255},
	"mediumslateblue":      color.RGBA{123, 104, 238, 255},
	"darkslateblue":        color.RGBA{72, 61, 139, 255},
	"mediumpurple":         color.RGBA{147, 112, 219, 255},
	"blueviolet":           color.RGBA{138, 43, 226, 255},
	"indigo":               color.RGBA{75, 0, 130, 255},
	"violet":               color.RGBA{238, 130, 238, 255},
	"orchid":               color.RGBA{218, 112, 214, 255},
	"plum":                 color.RGBA{221, 160, 221, 255},
	"mediumorchid":         color.RGBA{186, 85, 211, 255},
	"darkorchid":           color.RGBA{153, 50, 204, 255},
	"darkviolet":           color.RGBA{148, 0, 211, 255},
	"darkmagenta":          color.RGBA{139, 0, 139, 255},
	"mediumvioletred":      color.RGBA{199, 21, 133, 255},
	"palevioletred":        color.RGBA{219, 112, 147, 255},
	"crimson":              color.RGBA{220, 20, 60, 255},
	"firebrick":            color.RGBA{178, 34, 34, 255},
	"indianred":            color.RGBA{205, 92, 92, 255},
	"tomato":               color.RGBA{255, 99, 71, 255},
	"salmon":               color.RGBA{250, 128, 114, 255},
	"coral":                color.RGBA{255, 127, 80, 255},
	"darksalmon":           color.RGBA{233, 150, 122, 255},
	"gold":                 color.RGBA{255, 215, 0, 255},
	"goldenrod":            color.RGBA{218, 165, 32, 255},
	"darkgoldenrod":        color.RGBA{184, 134, 11, 255},
	"khaki":                color.RGBA{240, 230, 140, 255},
	"darkkhaki":            color.RGBA{189, 183, 107, 255},
	"palegoldenrod":        color.RGBA{238, 232, 170, 255},
	"lemonchiffon":         color.RGBA{255, 250, 205, 255},
	"lightgoldenrodyellow": color.RGBA{250, 250, 210, 255},
	"wheat":                color.RGBA{245, 222, 179, 255},
	"burlywood":            color.RGBA{222, 184, 135, 255},
	"tan":                  color.RGBA{210, 180, 140, 255},
	"sandybrown":           color.RGBA{244, 164, 96, 255},
	"peru":                 color.RGBA{205, 133, 63, 255},
	"chocolate":            color.RGBA{210, 105, 30, 255},
	"sienna":               color.RGBA{160, 82, 45, 255},
	"rosybrown":            color.RGBA{188, 143, 143, 255},
	"moccasin":             color.RGBA{255, 228, 181, 255},
	"peachpuff":            color.RGBA{255, 218, 185, 255},
	"bisque":               color.RGBA{255, 228, 196, 255},
	"antiquewhite":         color.RGBA{250, 235, 215, 255},
	"linen":                color.RGBA{250, 240, 230, 255},
	"oldlace":              color.RGBA{253, 245, 230, 255},
	"floralwhite":          color.RGBA{255, 250, 240, 255},
	"ivory":                color.RGBA{255, 255, 240, 255},
	"seashell":             color.RGBA{255, 245, 238, 255},
	"lavenderblush":        color.RGBA{255, 240, 245, 255},
	"mistyrose":            color.RGBA{255, 228, 225, 255},
	"snow":                 color.RGBA{255, 250, 250, 255},
	"mintcream":            color.RGBA{245, 255, 250, 255},
	"honeydew":             color.RGBA{240, 255, 240, 255},
	"azure":                color.RGBA{240, 255, 255, 255},
	"ghostwhite":           color.RGBA{248, 248, 255, 255},
	"lavender":             color.RGBA{230, 230, 250, 255},
	"thistle":              color.RGBA{216, 191, 216, 255},
	"gainsboro":            color.RGBA{220, 220, 220, 255},
	"whitesmoke":           color.RGBA{245, 245, 245, 255},
	"beige":                color.RGBA{245, 245, 220, 255},
	"cornsilk":             color.RGBA{255, 248, 220, 255},
	"blanchedalmond":       color.RGBA{255, 235, 205, 255},
	"navajowhite":          color.RGBA{255, 222, 173, 255},
	"papayawhip":           color.RGBA{255, 239, 213, 255},
	"springgreen":          color.RGBA{0, 255, 127, 255},
	"mediumspringgreen":    color.RGBA{0, 250, 154, 255},
	"lawngreen":            color.RGBA{124, 252, 0, 255},
	"chartreuse":           color.RGBA{127, 255, 0, 255},
	"greenyellow":          color.RGBA{173, 255, 47, 255},
	"yellowgreen":          color.RGBA{154, 205, 50, 255},
	"olivedrab":            color.RGBA{107, 142, 35, 255},
	"darkolivegreen":       color.RGBA{85, 107, 47, 255},
	"palegreen":            color.RGBA{152, 251, 152, 255},
	"mediumseagreen":       color.RGBA{60, 179, 113, 255},
	"seagreen":             color.RGBA{46, 139, 87, 255},
	"darkseagreen":         color.RGBA{143, 188, 143, 255},
	"forestgreen":          color.RGBA{34, 139, 34, 255},
	"mediumaquamarine":     color.RGBA{102, 205, 170, 255},
	"aquamarine":           color.RGBA{127, 255, 212, 255},
	"turquoise":            color.RGBA{64, 224, 208, 255},
	"mediumturquoise":      color.RGBA{72, 209, 204, 255},
	"darkturquoise":        color.RGBA{0, 206, 209, 255},
	"darkcyan":             color.RGBA{0, 139, 139, 255},
	"transparent":          color.RGBA{0, 0, 0, 0},
}
