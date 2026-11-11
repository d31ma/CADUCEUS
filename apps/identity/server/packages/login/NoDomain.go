package login

type NoDomain struct{}

func (NoDomain) Error() string {
	return "the person has no domain"
}
