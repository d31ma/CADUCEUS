type AuthDecision struct {
	Allowed   bool      `json:"allowed"`
	Principal Principal `json:"principal"`
	Domain    string    `json:"domain"`
}
