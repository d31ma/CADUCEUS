type PasswordPolicy struct {
	Rule string
}

func (failure *PasswordPolicy) Error() string {
	return failure.Rule
}
