package reception

type UnknownAddress struct{}

func (UnknownAddress) Error() string {
	return "the address is not known in a hosted domain"
}
