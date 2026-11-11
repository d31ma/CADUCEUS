type MemberQuery struct {
	Text   string `json:"text"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}
