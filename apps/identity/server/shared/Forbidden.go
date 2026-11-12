type Forbidden struct {
	ErrorCase
}

func (Forbidden) NotAMember() *Forbidden {
	return &Forbidden{ErrorCase{505, "not_a_member", "", "You are not a member of that domain."}}
}

func (Forbidden) NoDomain() *Forbidden {
	return &Forbidden{ErrorCase{506, "no_domain", "", "This account may not open that domain."}}
}

func (Forbidden) AccessRevoked() *Forbidden {
	return &Forbidden{ErrorCase{507, "access_revoked", "", "Your access was removed. Sign in again."}}
}
