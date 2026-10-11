type DiscoveredSettings struct {
	ChoiceID   string               `json:"choiceId"`
	Source     string               `json:"source"`
	Label      string               `json:"label"`
	Connection DiscoveredConnection `json:"connection"`
}
