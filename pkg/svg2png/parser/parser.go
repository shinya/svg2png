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
