type LoginState struct {
	TransactionID   string   `json:"transactionId"`
	RequiredFactors []string `json:"requiredFactors"`
	Satisfied       bool     `json:"satisfied"`
	State           string   `json:"state"`
}
