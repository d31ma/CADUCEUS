package inbound

type AddressKinds struct{}

func (AddressKinds) Member() string {
	return "member"
}

func (AddressKinds) Alias() string {
	return "alias"
}

func (AddressKinds) Channel() string {
	return "channel"
}
