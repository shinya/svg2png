package font

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/image/font/sfnt"
)

// FontInfo はフォントの情報を表します
type FontInfo struct {
	Family string
	Style  string
	Path   string
	Data   []byte
}

// Manager はフォントの管理を行います
type Manager struct {
	fonts    map[string]map[string]*FontInfo // family -> style -> FontInfo
	renderer *Renderer
	mu       sync.RWMutex
}

// NewManager は新しいフォントマネージャーを作成します
func NewManager() *Manager {
	return &Manager{
		fonts:    make(map[string]map[string]*FontInfo),
		renderer: NewRenderer(),
	}
}

// RegisterFonts はフォントを登録します
func (m *Manager) RegisterFonts(fonts ...FontSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, font := range fonts {
		if font.Family == "" {
			return fmt.Errorf("font family cannot be empty")
		}

		// スタイルの正規化
		style := normalizeStyle(font.Style)

		// ファミリの初期化
		if m.fonts[font.Family] == nil {
			m.fonts[font.Family] = make(map[string]*FontInfo)
		}

		// フォント情報の作成
		fontInfo := &FontInfo{
			Family: font.Family,
			Style:  style,
			Path:   font.Path,
			Data:   font.Data,
		}

		m.fonts[font.Family][style] = fontInfo

		log.Printf("Font registered: %s %s from %s", font.Family, style, font.Path)

		// フォントレンダラーにフォントを読み込み
		if err := m.renderer.LoadFont(fontInfo); err != nil {
			return fmt.Errorf("failed to load font %s %s: %w", font.Family, style, err)
		}
	}

	return nil
}

// ClearCache はフォントキャッシュをクリアします
func (m *Manager) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fonts = make(map[string]map[string]*FontInfo)
	m.renderer = NewRenderer()
}

// GetFont は指定されたファミリとスタイルのフォントを取得します
func (m *Manager) GetFont(family, style string) (*FontInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// スタイルの正規化
	style = normalizeStyle(style)

	// 直接マッチ
	if fonts, exists := m.fonts[family]; exists {
		if font, exists := fonts[style]; exists {
			return font, nil
		}
	}

	// フォールバック: Regularスタイルを試す
	if fonts, exists := m.fonts[family]; exists {
		if font, exists := fonts["Regular"]; exists {
			return font, nil
		}
	}

	// フォールバック: 類似のファミリ名を試す
	if similarFamily := m.findSimilarFamily(family); similarFamily != "" {
		if fonts, exists := m.fonts[similarFamily]; exists {
			if font, exists := fonts[style]; exists {
				return font, nil
			}
			if font, exists := fonts["Regular"]; exists {
				return font, nil
			}
		}
	}

	return nil, fmt.Errorf("font not found: %s %s", family, style)
}

// findSimilarFamily は類似のフォントファミリ名を探します
func (m *Manager) findSimilarFamily(family string) string {
	family = strings.ToLower(family)

	// 一般的なフォントファミリ名のマッピング
	fontMappings := map[string]string{
		"arial":      "Arial Unicode",
		"helvetica":  "Geneva",
		"times":      "Tinos",
		"courier":    "Monaco",
		"sans-serif": "Geneva",
		"serif":      "Tinos",
		"monospace":  "Monaco",
	}

	if mapped, exists := fontMappings[family]; exists {
		// マッピングされたフォントが存在するかチェック
		if fonts, exists := m.fonts[mapped]; exists && len(fonts) > 0 {
			return mapped
		}
	}

	// 部分一致で探す
	for registeredFamily := range m.fonts {
		if strings.Contains(strings.ToLower(registeredFamily), family) {
			return registeredFamily
		}
	}

	return ""
}

// GetRenderer はフォントレンダラーを取得します
func (m *Manager) GetRenderer() *Renderer {
	return m.renderer
}

// ListFonts は登録されているフォントの一覧を返します
func (m *Manager) ListFonts() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var fonts []string
	for family, styles := range m.fonts {
		for style := range styles {
			fonts = append(fonts, fmt.Sprintf("%s %s", family, style))
		}
	}
	return fonts
}

// ScanSystemFonts はシステムフォントをスキャンします
func (m *Manager) ScanSystemFonts() error {
	paths := getSystemFontPaths()

	log.Printf("Scanning system fonts from paths: %v", paths)

	for _, path := range paths {
		if err := m.scanDirectory(path); err != nil {
			// 警告として記録するが、処理は続行
			log.Printf("Warning: failed to scan directory %s: %v", path, err)
		}
	}

	log.Printf("System font scan completed. Registered fonts: %v", m.ListFonts())
	return nil
}

// scanDirectory は指定されたディレクトリ内のフォントをスキャンします
func (m *Manager) scanDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ttf" && ext != ".otf" && ext != ".ttc" {
			return nil
		}

		if ext == ".ttc" {
			// TrueType Collection: 複数フォントを含むファイル
			m.scanTTCFile(path)
			return nil
		}

		// 単一フォントファイル: メタデータから情報を抽出
		family, style, err := extractFontInfoFromFile(path)
		if err != nil {
			// フォールバック: ファイル名から推測
			family, style, err = extractFontInfo(path)
			if err != nil {
				return nil
			}
		}

		// フォントの登録
		font := FontSource{
			Family: family,
			Style:  style,
			Path:   path,
		}

		if err := m.RegisterFonts(font); err != nil {
			log.Printf("Warning: skipping font %s: %v", path, err)
		}
		return nil // 個別フォントのエラーでスキャンを止めない
	})
}

// getSystemFontPaths はプラットフォーム別のフォントパスを返します
func getSystemFontPaths() []string {
	switch runtime.GOOS {
	case "linux":
		return []string{
			"/usr/share/fonts",
			"/usr/local/share/fonts",
			filepath.Join(os.Getenv("HOME"), ".local/share/fonts"),
		}
	case "darwin":
		return []string{
			"/System/Library/Fonts",
			"/Library/Fonts",
			filepath.Join(os.Getenv("HOME"), "Library/Fonts"),
		}
	case "windows":
		return []string{
			filepath.Join(os.Getenv("WINDIR"), "Fonts"),
		}
	default:
		return []string{}
	}
}

// normalizeStyle はスタイル名を正規化します
func normalizeStyle(style string) string {
	style = strings.ToLower(style)

	switch {
	case strings.Contains(style, "bold") && strings.Contains(style, "italic"):
		return "BoldItalic"
	case strings.Contains(style, "bold"):
		return "Bold"
	case strings.Contains(style, "italic"):
		return "Italic"
	default:
		return "Regular"
	}
}

// scanTTCFile はTrueType Collectionファイルをスキャンして個別フォントを登録します
func (m *Manager) scanTTCFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Warning: failed to read TTC file %s: %v", path, err)
		return
	}

	collection, err := sfnt.ParseCollection(data)
	if err != nil {
		log.Printf("Warning: failed to parse TTC file %s: %v", path, err)
		return
	}

	numFonts := collection.NumFonts()
	for i := 0; i < numFonts; i++ {
		f, err := collection.Font(i)
		if err != nil {
			log.Printf("Warning: failed to get font %d from %s: %v", i, path, err)
			continue
		}

		family, style := extractFontInfoFromSFNT(f)
		if family == "" {
			continue
		}

		key := fmt.Sprintf("%s-%s", family, normalizeStyle(style))
		if _, exists := m.renderer.fonts[key]; exists {
			continue // 既に登録済み
		}

		// TTC用の特別な登録（sfnt.Fontを直接使用）
		if err := m.registerTTCFont(family, style, f, data, i); err != nil {
			log.Printf("Warning: skipping TTC font %s/%s from %s: %v", family, style, path, err)
		}
	}
}

// registerTTCFont はTTCファイル内の個別フォントを登録します
func (m *Manager) registerTTCFont(family, style string, f *sfnt.Font, ttcData []byte, index int) error {
	normalizedStyle := normalizeStyle(style)

	if m.fonts[family] == nil {
		m.fonts[family] = make(map[string]*FontInfo)
	}

	fontInfo := &FontInfo{
		Family: family,
		Style:  normalizedStyle,
	}
	m.fonts[family][normalizedStyle] = fontInfo

	// レンダラーに直接登録（opentype.ParseCollectionを使用）
	return m.renderer.LoadFontFromCollection(family, normalizedStyle, ttcData, index)
}

// extractFontInfoFromFile はフォントファイルのメタデータからファミリ名とスタイルを抽出します
func extractFontInfoFromFile(path string) (family, style string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	f, err := sfnt.Parse(data)
	if err != nil {
		return "", "", err
	}
	family, style = extractFontInfoFromSFNT(f)
	if family == "" {
		return "", "", fmt.Errorf("could not extract font family name")
	}
	return family, style, nil
}

// extractFontInfoFromSFNT はsfnt.Fontからファミリ名とスタイルを抽出します
func extractFontInfoFromSFNT(f *sfnt.Font) (family, style string) {
	var buf sfnt.Buffer

	// ファミリ名を取得（NameIDFamily = 1）
	if name, err := f.Name(&buf, sfnt.NameIDFamily); err == nil && name != "" {
		family = name
	}
	// サブファミリ（スタイル）を取得（NameIDSubfamily = 2）
	if name, err := f.Name(&buf, sfnt.NameIDSubfamily); err == nil && name != "" {
		style = name
	}

	if style == "" {
		style = "Regular"
	}
	return family, style
}

// extractFontInfo はフォントファイルから情報を抽出します（簡易版、ファイル名ベース）
func extractFontInfo(path string) (family, style string, err error) {
	// 実際の実装では、TTF/OTFファイルのnameテーブルを読み取る
	// ここでは簡易的にファイル名から推測
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	// スタイルの推測（Bold+Italicの組み合わせを先に確認）
	nameLower := strings.ToLower(name)
	isBold := strings.Contains(nameLower, "bold")
	isItalic := strings.Contains(nameLower, "italic") || strings.Contains(nameLower, "oblique")

	switch {
	case isBold && isItalic:
		style = "BoldItalic"
	case isBold:
		style = "Bold"
	case isItalic:
		style = "Italic"
	default:
		style = "Regular"
	}

	// ファミリ名の推測（スタイル文字列を除去）
	family = name
	for _, suffix := range []string{"Bold Italic", "BoldItalic", "Bold", "Italic", "Oblique", "Regular"} {
		// 大文字小文字を区別しない除去
		idx := strings.Index(strings.ToLower(family), strings.ToLower(suffix))
		if idx >= 0 {
			family = family[:idx] + family[idx+len(suffix):]
		}
	}
	family = strings.TrimSpace(family)
	// 連続するスペースを1つに
	for strings.Contains(family, "  ") {
		family = strings.ReplaceAll(family, "  ", " ")
	}

	if family == "" {
		family = "Unknown"
	}

	return family, style, nil
}
