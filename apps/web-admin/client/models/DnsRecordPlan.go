type DnsRecordPlan struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Need     string `json:"need"`
	Host     string `json:"host"`
	Selector string `json:"selector"`
	State    string `json:"state"`
}
