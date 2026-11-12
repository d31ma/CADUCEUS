import (
	"encoding/json"
	"os"
	"time"

	"mesail.dev/packages/mailout"
	"mesail.dev/packages/oidc"
)

// @TestSetup
type MailOutTestSetup struct{}

func (MailOutTestSetup) variable() string { return "YON_APPLICATION_JSON" }

func (MailOutTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (setup MailOutTestSetup) Use(settings mailout.Settings, services string) {
	signing, _ := oidc.SigningRealm{}.StartedMesail()
	identity := signing.Settings
	identity.ServiceTokens = services + "/api/v1/identity/service-token"
	snapshot := map[string]any{"yon": map[string]any{
		"mailout": settings,
		"outbox": map[string]any{
			"producer":    "mail-out",
			"subscribers": []map[string]string{{"name": "messaging", "url": services + "/api/v1/messaging/events"}},
			"retry":       map[string]int{"baseDelaySeconds": 1, "maxDelaySeconds": 1, "maxAttempts": 2},
		},
		"oidc":    identity,
		"service": map[string]any{"client": map[string]string{"secret": "fixture-client-value"}},
	}}
	text, _ := json.Marshal(snapshot)
	os.Setenv(setup.variable(), string(text))
}

func (setup MailOutTestSetup) Realm() oidc.SigningRealm {
	folder, _ := os.MkdirTemp("", "mail-out-tokens-")
	os.Setenv("TMPDIR", folder)
	realm, _ := oidc.SigningRealm{}.StartedMesail()
	realm.Publish(time.Now())
	return realm
}

func (MailOutTestSetup) ServiceToken(realm oidc.SigningRealm) string {
	token, _ := realm.Token(oidc.TokenRequest{Subject: "service-account", RealmRoles: []string{"mesail-service"}, Audience: "mesail-api", AuthorizedParty: "mesail-service", ExpiresAt: time.Now().Unix() + 60})
	return token
}
