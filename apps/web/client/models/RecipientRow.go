type RecipientRow struct {
	Role        string      `json:"role"`
	Label       string      `json:"label"`
	Placeholder string      `json:"placeholder"`
	Text        string      `json:"text"`
	Recipients  []Recipient `json:"recipients"`
}
