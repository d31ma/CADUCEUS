type DomainStateLook struct {
	Text         string `json:"text"`
	TagClass     string `json:"tagClass"`
	IconClass    string `json:"iconClass"`
	Note         string `json:"note"`
	RowClass     string `json:"rowClass"`
	ActionText   string `json:"actionText"`
	ActionClass  string `json:"actionClass"`
	ActionPrefix string `json:"actionPrefix"`
	Pending      bool   `json:"pending"`
}
