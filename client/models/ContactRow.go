// ContactRow is one row of the contact list, computed for the view.
type ContactRow struct {
	ID          string `json:"id"`
	Heading     string `json:"heading"`
	Subtitle    string `json:"subtitle"`
	Initials    string `json:"initials"`
	SourceLabel string `json:"sourceLabel"`
	SourceClass string `json:"sourceClass"`
	ClassName   string `json:"className"`
	Selected    string `json:"selected"`
	Checked     bool   `json:"checked"`
	SelectLabel string `json:"selectLabel"`
}
