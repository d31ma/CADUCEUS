package keycloak

import (
	"net/url"
	"strings"
)

type adminAddresses struct {
	base string
}

func (addresses adminAddresses) Token() string {
	return addresses.trimmedBase() + "/realms/master/protocol/openid-connect/token"
}

func (addresses adminAddresses) Realms() string {
	return addresses.trimmedBase() + "/admin/realms"
}

func (addresses adminAddresses) Realm(realm string) string {
	return addresses.Realms() + "/" + url.PathEscape(realm)
}

func (addresses adminAddresses) trimmedBase() string {
	return strings.TrimSuffix(addresses.base, "/")
}
