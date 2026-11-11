package login

type NotAMember struct{}

func (NotAMember) Error() string {
	return "the person is not a member of this domain"
}
