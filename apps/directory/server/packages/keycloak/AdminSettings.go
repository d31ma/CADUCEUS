package keycloak

type AdminSettings struct {
	BaseUrl      string
	ClientId     string
	ClientSecret string
	Secrets      RealmSecrets
}
