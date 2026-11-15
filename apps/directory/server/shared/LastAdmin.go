type LastAdmin struct {
	ErrorCase
}

func (LastAdmin) LastAdmin() *LastAdmin {
	return &LastAdmin{ErrorCase{1034, "last_admin", "", "a domain must keep at least one admin"}}
}
