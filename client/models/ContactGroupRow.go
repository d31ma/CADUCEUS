// ContactGroupRow is one group of the sidebar with its actions.
type ContactGroupRow struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ClassName      string `json:"className"`
	RenameLabel    string `json:"renameLabel"`
	DeleteLabel    string `json:"deleteLabel"`
	DeleteQuestion string `json:"deleteQuestion"`
}
