type ContextMenuState struct {
	Open      bool     `json:"open"`
	Targets   []string `json:"targets"`
	X         float64  `json:"x"`
	Y         float64  `json:"y"`
	Heading   string   `json:"heading"`
	ReadLabel string   `json:"readLabel"`
	StarLabel string   `json:"starLabel"`
}
