type CategoryAccount struct {
	Domain       string               `json:"domain"`
	Capabilities CategoryCapabilities `json:"capabilities"`
	Categories   []Category           `json:"categories"`
	Mappings     map[string]string    `json:"mappings"`
}
