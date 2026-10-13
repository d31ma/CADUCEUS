type ThemeOption struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	BaseMode    string   `json:"baseMode"`
	Colors      []string `json:"colors"`
}
