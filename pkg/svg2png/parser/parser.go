package parser

import (
	"encoding/xml"
	"fmt"
	"log"
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
	log.Printf("SVG parsing started, data length: %d bytes", len(data))

	var svg struct {
		XMLName xml.Name `xml:"svg"`
		ViewBox string   `xml:"viewBox,attr"`
		Width   string   `xml:"width,attr"`
		Height  string   `xml:"height,attr"`
		Content []byte   `xml:",innerxml"`
	}

	if err := xml.Unmarshal(data, &svg); err != nil {
		return nil, fmt.Errorf("failed to parse SVG: %w", err)
	}

	log.Printf("SVG root parsed: viewBox=%s, width=%s, height=%s, content length=%d",
		svg.ViewBox, svg.Width, svg.Height, len(svg.Content))

	// viewBoxの解析
	var viewBox *ViewBox
	if svg.ViewBox != "" {
		if vb, err := parseViewBox(svg.ViewBox); err == nil {
			viewBox = vb
			log.Printf("ViewBox parsed: x=%f, y=%f, width=%f, height=%f",
				vb.X, vb.Y, vb.Width, vb.Height)
		}
	}

	// ルート要素の作成
	root := &Element{
		Name:       "svg",
		Attributes: make(map[string]string),
	}

	// 属性の解析
	if svg.Width != "" {
		root.Attributes["width"] = svg.Width
	}
	if svg.Height != "" {
		root.Attributes["height"] = svg.Height
	}
	if svg.ViewBox != "" {
		root.Attributes["viewBox"] = svg.ViewBox
	}

	// 子要素の解析
	children, err := parseElements(svg.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse child elements: %w", err)
	}
	root.Children = children

	log.Printf("SVG parsing completed: %d child elements found", len(children))

	return &Document{
		Root:    root,
		ViewBox: viewBox,
		Width:   svg.Width,
		Height:  svg.Height,
		DPI:     96, // デフォルト値
	}, nil
}

// parseElements はXMLコンテンツから要素を解析します
func parseElements(content []byte) ([]*Element, error) {
	var elements []*Element

	log.Printf("Parsing child elements from content length: %d", len(content))

	// 簡易的な要素解析（実際の実装ではより詳細な解析が必要）
	// ここでは基本的な構造のみを実装
	decoder := xml.NewDecoder(strings.NewReader(string(content)))

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			elem := &Element{
				Name:       t.Name.Local,
				Attributes: make(map[string]string),
				Children:   []*Element{},
			}

			// 属性の解析
			for _, attr := range t.Attr {
				elem.Attributes[attr.Name.Local] = attr.Value
			}

			log.Printf("Element found: %s with %d attributes", elem.Name, len(elem.Attributes))
			elements = append(elements, elem)
		case xml.CharData:
			// テキストコンテンツの処理
			text := strings.TrimSpace(string(t))
			if text != "" && len(elements) > 0 {
				elements[len(elements)-1].Text = text
				log.Printf("Text content found: '%s'", text)
			}
		}
	}

	log.Printf("Child elements parsing completed: %d elements found", len(elements))
	return elements, nil
}

// parseViewBox はviewBox属性を解析します
func parseViewBox(viewBox string) (*ViewBox, error) {
	parts := strings.Fields(viewBox)
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid viewBox format: %s", viewBox)
	}

	x, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid x value in viewBox: %w", err)
	}

	y, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid y value in viewBox: %w", err)
	}

	width, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid width value in viewBox: %w", err)
	}

	height, err := strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid height value in viewBox: %w", err)
	}

	return &ViewBox{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
	}, nil
}
