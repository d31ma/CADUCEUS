type PasswordPolicy struct {
	ErrorCase
}

func (PasswordPolicy) Refused(rule string) *PasswordPolicy {
	return &PasswordPolicy{ErrorCase{1031, "password_policy", "password", rule}}
}
