type ShortcutValidation struct {
	Valid     bool              `json:"valid"`
	Problem   string            `json:"problem"`
	Overrides map[string]string `json:"overrides"`
}
