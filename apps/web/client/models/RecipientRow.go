type RecipientRow struct {
	Role        string      `json:"role"`
	Label       string      `json:"label"`
	Placeholder string      `json:"placeholder"`
	Recipients  []Recipient `json:"recipients"`
	Shown       bool        `json:"shown"`
}
