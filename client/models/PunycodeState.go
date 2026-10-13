type PunycodeState struct {
	CodePoint int    `json:"codePoint"`
	Delta     int    `json:"delta"`
	Bias      int    `json:"bias"`
	Handled   int    `json:"handled"`
	Basic     int    `json:"basic"`
	Output    string `json:"output"`
}
