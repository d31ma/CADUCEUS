type ActivitySubject struct {
	Subject string  `json:"subject"`
	Sent    float64 `json:"sent"`
	Opened  float64 `json:"opened"`
	Clicked float64 `json:"clicked"`
	Replied float64 `json:"replied"`
}
