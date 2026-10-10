type AccountSession struct {
	Token   string   `json:"token"`
	Email   string   `json:"email"`
	Role    string   `json:"role"`
	Domains []string `json:"domains"`
}
