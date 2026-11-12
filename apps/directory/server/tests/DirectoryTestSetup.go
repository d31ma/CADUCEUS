import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"mesail.dev/packages/oidc"
)

// @TestSetup
type DirectoryTestSetup struct{}

func (DirectoryTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (setup DirectoryTestSetup) SignedRealm() oidc.SigningRealm {
	folder, err := os.MkdirTemp("", "directory-test-")
	setup.must(err)
	os.Setenv("TMPDIR", folder)
	realm, err := oidc.SigningRealm{}.StartedMesail()
	setup.must(err)
	setup.must(realm.Publish(time.Now()))
	return realm
}

func (setup DirectoryTestSetup) ApplicationJson(realm oidc.SigningRealm, keycloakBase string) string {
	settings, err := json.Marshal(realm.Settings)
	setup.must(err)
	directory := `"directory":{"keycloakurl":%q,"admin":{"client":"mesail-directory","secret":"example-secret"}}`
	return `{"yon":{"oidc":` + string(settings) + `,` + fmt.Sprintf(directory, keycloakBase+"/idp") + `}}`
}

func (setup DirectoryTestSetup) PersonBearer(realm oidc.SigningRealm, subject string, domainId string, realmRoles ...string) string {
	request := oidc.TokenRequest{Subject: subject, Name: "Test Caller", Username: "caller", Email: "caller@" + domainId, Domain: domainId, RealmRoles: realmRoles, Audience: setup.audience(), AuthorizedParty: "mesail-web", ExpiresAt: setup.expiry()}
	return setup.bearer(realm, request)
}

func (setup DirectoryTestSetup) ServiceBearer(realm oidc.SigningRealm, domainId string) string {
	request := oidc.TokenRequest{Subject: "service", Domain: domainId, RealmRoles: []string{"mesail-service"}, Audience: setup.audience(), AuthorizedParty: "mesail-service", ExpiresAt: setup.expiry()}
	return setup.bearer(realm, request)
}

func (setup DirectoryTestSetup) bearer(realm oidc.SigningRealm, request oidc.TokenRequest) string {
	token, err := realm.Token(request)
	setup.must(err)
	return "Bearer " + token
}

func (DirectoryTestSetup) audience() string {
	return "mesail-api"
}

func (DirectoryTestSetup) expiry() int64 {
	return time.Now().Add(time.Minute).Unix()
}

func (DirectoryTestSetup) must(err error) {
	Assert.Equals("<nil>", fmt.Sprint(err))
}
