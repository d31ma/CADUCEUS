// KeyboardAction is one command of the keyboard catalog.
type KeyboardAction struct {
	Id      string `json:"id"`
	Scope   string `json:"scope"`
	Binding string `json:"binding"`
}
