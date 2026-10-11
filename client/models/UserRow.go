type UserRow struct {
	Email           string `json:"email"`
	Role            string `json:"role"`
	Note            string `json:"note"`
	OperatorCommand string `json:"operatorCommand"`
	HasRecovery     bool   `json:"hasRecovery"`
}
