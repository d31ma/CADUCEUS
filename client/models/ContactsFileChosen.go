// ContactsFileChosen is the vCard file picked for import, read as text.
type ContactsFileChosen struct {
	Name    string `json:"name"`
	Size    int    `json:"size"`
	Content string `json:"content"`
}
