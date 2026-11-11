package keycloak

type newUserBody struct {
	Username    string               `json:"username"`
	Enabled     bool                 `json:"enabled"`
	Attributes  map[string][]string  `json:"attributes"`
	Credentials []passwordCredential `json:"credentials"`
}
