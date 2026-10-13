// VCardProperty is one content line of a vCard: its upper-case name, its
// raw value and its first TYPE parameter in lower case ("" for none).
type VCardProperty struct {
	Name  string
	Value string
	Type  string
}
