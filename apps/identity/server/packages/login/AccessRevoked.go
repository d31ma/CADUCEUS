package login

type AccessRevoked struct{}

func (AccessRevoked) Error() string {
	return "the person's access was revoked"
}
