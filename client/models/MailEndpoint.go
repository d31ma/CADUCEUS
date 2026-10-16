type MailEndpoint struct {
	Host             string  `json:"host"`
	Port             float64 `json:"port"`
	Security         string  `json:"security"`
	Username         string  `json:"username"`
	AllowInsecureTLS bool    `json:"allowInsecureTLS"`
}
