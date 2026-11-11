package keycloak

type addRequest struct{}

func (addRequest) Key() string {
	return "mesail.addRequest"
}

func (addRequest) Values(value string) []string {
	return []string{value}
}
