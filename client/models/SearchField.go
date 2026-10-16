// SearchField is one field name of the search syntax, such as from: or is:.
type SearchField struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Values      []string `json:"values"`
	Shown       bool     `json:"shown"`
}
