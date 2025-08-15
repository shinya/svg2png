package font

// FontSource はフォントの供給源を表します
type FontSource struct {
	Family string // ファミリ名（例: "Noto Sans CJK JP"）
	Style  string // "Regular","Italic","Bold","BoldItalic"
	Data   []byte // TTF/OTF (任意: メモリ登録用)
	Path   string // ファイル登録用（Data or Path のいずれか）
}
