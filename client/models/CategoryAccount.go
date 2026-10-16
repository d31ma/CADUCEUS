type CategoryAccount struct {
	Domain       string               `json:"domain"`
	Capabilities CategoryCapabilities `json:"capabilities"`
	Categories   []Category           `json:"categories"`
}
