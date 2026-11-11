package keycloak

type NotConfigured struct{}

func (NotConfigured) Error() string {
	return "the Keycloak user client is not configured: the base URL, client id and client secret are required"
}
