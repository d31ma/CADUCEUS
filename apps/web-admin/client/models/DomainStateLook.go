type DomainStateLook struct {
	Text          string `json:"text"`
	Color         string `json:"color"`
	Variant       string `json:"variant"`
	Icon          string `json:"icon"`
	Note          string `json:"note"`
	Tone          string `json:"tone"`
	ActionText    string `json:"actionText"`
	ActionVariant string `json:"actionVariant"`
	ActionPrefix  string `json:"actionPrefix"`
	Pending       bool   `json:"pending"`
	Locked        bool   `json:"locked"`
}
