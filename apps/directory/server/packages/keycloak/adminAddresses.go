package keycloak

import "strings"

type adminAddresses struct {
	base string
}

func (addresses adminAddresses) Token() string {
	return addresses.trimmedBase() + "/realms/" + addresses.realm() + "/protocol/openid-connect/token"
}

func (addresses adminAddresses) Admin(path string) string {
	return addresses.trimmedBase() + "/admin/realms/" + addresses.realm() + path
}

func (addresses adminAddresses) trimmedBase() string {
	return strings.TrimSuffix(addresses.base, "/")
}

func (adminAddresses) realm() string {
	return "mesail"
}
