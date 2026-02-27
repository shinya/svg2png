package parser

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Document はSVGドキュメントを表します
type Document struct {
	Root    *Element
	ViewBox *ViewBox
	Width   string
	Height  string
	DPI     float64
	Defs    *Defs
}

// Element はSVG要素を表します
type Element struct {
	Name       string
	Attributes map[string]string
	Children   []*Element
	Text       string
}

// ViewBox はSVGのviewBox属性を表します
type ViewBox struct {
	X, Y, Width, Height float64
}

// Defs はSVG defs 内の定義を格納します
type Defs struct {
	LinearGradients map[string]*LinearGradient
	RadialGradients map[string]*RadialGradient
	ClipPaths       map[string]*Element // clipPath要素（子要素ごとレンダリングに使う）
	Patterns        map[string]*Element // pattern要素
}

// GradientStop はグラデーションのカラーストップです
type GradientStop struct {
	Offset  float64
	Color   string  // CSS色文字列
	Opacity float64 // stop-opacity (0-1)
}

// LinearGradient は線形グラデーション定義です
type LinearGradient struct {
	ID            string
	X1, Y1, X2, Y2 string
	GradientUnits string // "objectBoundingBox" | "userSpaceOnUse"
	Stops         []GradientStop
}

// RadialGradient は放射状グラデーション定義です
type RadialGradient struct {
	ID            string
	CX, CY, R     string
	FX, FY        string
	GradientUnits string
	Stops         []GradientStop
}

// ParseSVG はSVGデータをパースします
func ParseSVG(data []byte) (*Document, error) {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose

	// SVGルート要素を探す
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("no SVG root element found")
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse SVG: %w", err)
		}

		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local == "svg" {
				root, err := parseElement(decoder, se)
				if err != nil {
					return nil, fmt.Errorf("failed to parse SVG root: %w", err)
				}

				doc := &Document{
					Root:   root,
					Width:  root.Attributes["width"],
					Height: root.Attributes["height"],
					DPI:    96,
				}

				if vbStr := root.Attributes["viewBox"]; vbStr != "" {
					if vb, err := parseViewBox(vbStr); err == nil {
						doc.ViewBox = vb
					}
				}

				doc.Defs = parseDefs(root)
				return doc, nil
			}
		}
	}
}

// parseElement はXMLデコーダーから要素を再帰的にパースします
func parseElement(decoder *xml.Decoder, start xml.StartElement) (*Element, error) {
	elem := &Element{
		Name:       start.Name.Local,
		Attributes: make(map[string]string),
	}

	// 属性の解析
	for _, attr := range start.Attr {
		// xmlns属性は無視
		if attr.Name.Space == "xmlns" || attr.Name.Local == "xmlns" {
			continue
		}
		elem.Attributes[attr.Name.Local] = attr.Value
	}

	// 子要素・テキストの再帰解析
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return elem, nil
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			child, err := parseElement(decoder, t)
			if err != nil {
				return nil, err
			}
			elem.Children = append(elem.Children, child)

		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" {
				elem.Text += text
			}

		case xml.EndElement:
			return elem, nil

		case xml.Comment:
			// コメントは無視
		case xml.ProcInst:
			// 処理命令は無視
		}
	}
}

// parseViewBox はviewBox属性を解析します
func parseViewBox(viewBox string) (*ViewBox, error) {
	// カンマまたはスペース区切りに対応
	viewBox = strings.ReplaceAll(viewBox, ",", " ")
	parts := strings.Fields(viewBox)
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid viewBox format: %s", viewBox)
	}

	vals := make([]float64, 4)
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid viewBox value %q: %w", p, err)
		}
		vals[i] = v
	}

	return &ViewBox{
		X:      vals[0],
		Y:      vals[1],
		Width:  vals[2],
		Height: vals[3],
	}, nil
}

// parseDefs はルート要素からdefs定義を抽出します
func parseDefs(root *Element) *Defs {
	defs := &Defs{
		LinearGradients: make(map[string]*LinearGradient),
		RadialGradients: make(map[string]*RadialGradient),
		ClipPaths:       make(map[string]*Element),
		Patterns:        make(map[string]*Element),
	}

	for _, child := range root.Children {
		if child.Name == "defs" {
			processDefsElement(defs, child)
		}
	}
	return defs
}

func processDefsElement(defs *Defs, defsElem *Element) {
	for _, def := range defsElem.Children {
		id := def.Attributes["id"]
		if id == "" {
			continue
		}
		switch def.Name {
		case "linearGradient":
			lg := &LinearGradient{
				ID:            id,
				X1:            def.Attributes["x1"],
				Y1:            def.Attributes["y1"],
				X2:            def.Attributes["x2"],
				Y2:            def.Attributes["y2"],
				GradientUnits: def.Attributes["gradientUnits"],
			}
			for _, stop := range def.Children {
				if stop.Name == "stop" {
					lg.Stops = append(lg.Stops, parseGradientStop(stop))
				}
			}
			defs.LinearGradients[id] = lg
		case "radialGradient":
			rg := &RadialGradient{
				ID:            id,
				CX:            def.Attributes["cx"],
				CY:            def.Attributes["cy"],
				R:             def.Attributes["r"],
				FX:            def.Attributes["fx"],
				FY:            def.Attributes["fy"],
				GradientUnits: def.Attributes["gradientUnits"],
			}
			for _, stop := range def.Children {
				if stop.Name == "stop" {
					rg.Stops = append(rg.Stops, parseGradientStop(stop))
				}
			}
			defs.RadialGradients[id] = rg
		case "clipPath":
			defs.ClipPaths[id] = def
		case "pattern":
			defs.Patterns[id] = def
		}
	}
}

// parseGradientStop はgradient stopをパースします
func parseGradientStop(stop *Element) GradientStop {
	s := GradientStop{Opacity: 1.0}

	// offset
	if offsetStr, ok := stop.Attributes["offset"]; ok {
		s.Offset = parseOffsetValue(offsetStr)
	}

	// style属性からstop-color, stop-opacity
	if styleStr, ok := stop.Attributes["style"]; ok {
		for _, kv := range strings.Split(styleStr, ";") {
			kv = strings.TrimSpace(kv)
			if kv == "" {
				continue
			}
			parts := strings.SplitN(kv, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			switch key {
			case "stop-color":
				s.Color = val
			case "stop-opacity":
				if v, err := strconv.ParseFloat(val, 64); err == nil {
					s.Opacity = v
				}
			}
		}
	}
	// プレゼンテーション属性（style属性の後でチェックするが、style属性が優先）
	if s.Color == "" {
		if c, ok := stop.Attributes["stop-color"]; ok {
			s.Color = c
		}
	}
	if oStr, ok := stop.Attributes["stop-opacity"]; ok {
		if v, err := strconv.ParseFloat(oStr, 64); err == nil {
			s.Opacity = v
		}
	}

	return s
}

// parseOffsetValue は "50%" または "0.5" 形式のオフセット値を 0-1 に変換します
func parseOffsetValue(s string) float64 {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err != nil {
			return 0
		}
		return v / 100.0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}
