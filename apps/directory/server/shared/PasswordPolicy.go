type PasswordPolicy struct {
	Message string
}

func (failure *PasswordPolicy) Error() string {
	return failure.Message
}
