type RuleAction struct {
	Type   string `json:"type"`
	Folder string `json:"folder,omitempty"`
	To     string `json:"to,omitempty"`
	Value  *bool  `json:"value,omitempty"`
}
