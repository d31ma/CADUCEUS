package login

import (
	"testing"

	"mesail.dev/packages/oidc"
)

func testIssuer(t *testing.T) ServiceTokenIssuer {
	t.Helper()
	return ServiceTokenIssuer{Check: oidc.ServiceClientCheck{Client: oidc.ClientSetting{Secret: "service-secret"}}, Signer: testSigner(t)}
}

func TestServiceTokenIssuerRefusesAnotherClient(t *testing.T) {
	_, err := testIssuer(t).Issue("mesail-web", "service-secret", "*")
	isError(t, err, oidc.SignInRefused{})
}

func TestServiceTokenIssuerRefusesAWrongSecret(t *testing.T) {
	_, err := testIssuer(t).Issue("mesail-service", "wrong", "*")
	isError(t, err, oidc.SignInRefused{})
}

func TestServiceTokenIssuerAcceptsTheWholeInstall(t *testing.T) {
	issued, err := testIssuer(t).Issue("mesail-service", "service-secret", "*")
	failOn(t, err)
	same(t, "domain", signedClaims(t, issued.AccessToken).Domain, "*")
}

func TestServiceTokenIssuerLowersADomainName(t *testing.T) {
	issued, err := testIssuer(t).Issue("mesail-service", "service-secret", "Example.COM")
	failOn(t, err)
	same(t, "domain", signedClaims(t, issued.AccessToken).Domain, "example.com")
}

func TestServiceTokenIssuerRefusesAnInvalidDomain(t *testing.T) {
	_, err := testIssuer(t).Issue("mesail-service", "service-secret", "not a domain")
	isError(t, err, InvalidDomain{})
}

func TestServiceTokenIssuerNamesTheServiceClient(t *testing.T) {
	issued, err := testIssuer(t).Issue("mesail-service", "service-secret", "example.com")
	failOn(t, err)
	claims := signedClaims(t, issued.AccessToken)
	same(t, "subject", claims.Subject, "service")
	same(t, "party", claims.AuthorizedParty, "mesail-service")
	same(t, "roles", claims.RealmRoles, []string{"mesail-service"})
}
