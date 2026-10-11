type TeamMember struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Label    string `json:"label"`
	Initials string `json:"initials"`
	Online   bool   `json:"online"`
	IsSelf   bool   `json:"isSelf"`
	Status   string `json:"status"`
}
