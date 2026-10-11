// EditableValue is one email, phone or website row of the contact editor.
// Key tells the rows apart while they are edited; it is never sent.
type EditableValue struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Value string `json:"value"`
}
