type DnsRecord struct {
	Key         string `json:"key"`
	TitleId     string `json:"titleId"`
	HostId      string `json:"hostId"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Need        string `json:"need"`
	Host        string `json:"host"`
	Selector    string `json:"selector"`
	HasSelector bool   `json:"hasSelector"`
	TagClass    string `json:"tagClass"`
	TagIcon     string `json:"tagIcon"`
	TagText     string `json:"tagText"`
	CardClass   string `json:"cardClass"`
	ResultClass string `json:"resultClass"`
	ResultIcon  string `json:"resultIcon"`
	ResultText  string `json:"resultText"`
	CopyLabel   string `json:"copyLabel"`
	CopiedLabel string `json:"copiedLabel"`
}
