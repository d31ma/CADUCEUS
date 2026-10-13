type RouteCaller struct {
	Principal Principal `json:"principal"`
	Token     string    `json:"-"`
	Email     string    `json:"email"`
	Domain    string    `json:"domain"`
	Domains   []string  `json:"domains"`
}
