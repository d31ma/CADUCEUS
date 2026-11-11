package dnscheck

type StateNames struct{}

func (StateNames) Ok() string {
	return "ok"
}

func (StateNames) Missing() string {
	return "missing"
}

func (StateNames) Wrong() string {
	return "wrong"
}
