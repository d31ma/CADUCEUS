type BadPasskey struct {
	Code string
}

func (failure *BadPasskey) Error() string {
	return "the passkey could not be added; start again"
}
