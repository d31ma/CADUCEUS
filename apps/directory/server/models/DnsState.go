type DnsState struct {
	Mx    string `json:"mx"`
	Spf   string `json:"spf"`
	Dkim  string `json:"dkim"`
	Dmarc string `json:"dmarc"`
}
