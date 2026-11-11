type Prepared struct {
	Item     OutboundItem  `json:"item"`
	Raw      string        `json:"raw"`
	Selector string        `json:"selector"`
	Source   MessageSource `json:"source"`
	Refusal  string        `json:"refusal"`
}
