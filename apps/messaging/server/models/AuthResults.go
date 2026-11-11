type AuthResults struct {
	Spf   string `json:"spf"`
	Dkim  string `json:"dkim"`
	Dmarc string `json:"dmarc"`
}
