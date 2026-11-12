type InvalidLabel struct{}

func (failure *InvalidLabel) Error() string {
	return "a passkey label is 1 to 64 characters"
}

func (failure *InvalidLabel) Code() string {
	return "invalid_request"
}
