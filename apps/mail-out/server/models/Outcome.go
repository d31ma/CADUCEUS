type Outcome struct {
	Status    string `json:"status"`
	Transport string `json:"transport"`
	Code      int    `json:"code"`
	Enhanced  string `json:"enhanced"`
	Detail    string `json:"detail"`
}
