// ExtensionCommand is a plugin command button in a page slot.
type ExtensionCommand struct {
	ExtensionID string `json:"extensionId"`
	CommandID   string `json:"commandId"`
	Label       string `json:"label"`
	Title       string `json:"title"`
	Slot        string `json:"slot"`
	Disabled    bool   `json:"disabled"`
}
