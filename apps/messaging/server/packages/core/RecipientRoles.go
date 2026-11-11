package core

type RecipientRoles struct{}

func (RecipientRoles) To() string {
	return "to"
}

func (RecipientRoles) Cc() string {
	return "cc"
}

func (RecipientRoles) Bcc() string {
	return "bcc"
}

func (RecipientRoles) Member() string {
	return "member"
}
