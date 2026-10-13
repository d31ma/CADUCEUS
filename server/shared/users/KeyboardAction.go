// KeyboardAction is one command of the keyboard catalog.
type KeyboardAction struct {
	Id      string `json:"id"`
	Label   string `json:"label"`
	Scope   string `json:"scope"`
	Binding string `json:"binding"`
}
