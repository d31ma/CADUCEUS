type RecipientOption struct {
	Index    int    `json:"index"`
	OptionID string `json:"optionId"`
	Label    string `json:"label"`
	Detail   string `json:"detail"`
	Initials string `json:"initials"`
	Active   bool   `json:"active"`
}
