type KeymapAction struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Group        string `json:"group"`
	Scope        string `json:"scope"`
	Binding      string `json:"binding"`
	AllowInInput bool   `json:"allowInInput"`
}
