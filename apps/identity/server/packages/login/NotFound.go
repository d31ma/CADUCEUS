package login

type NotFound struct{}

func (NotFound) Error() string {
	return "the person is not a member of that domain"
}
