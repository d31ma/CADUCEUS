package login

import (
	"encoding/json"
	"os"
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/transport"
)

type SessionsFactory struct{}

func (factory SessionsFactory) Configured() Sessions {
	return factory.FromJson(os.Getenv(factory.variable()))
}

func (factory SessionsFactory) FromJson(document string) Sessions {
	return factory.realm(document)
}

func (factory SessionsFactory) ServiceTokens(document string) ServiceTokenIssuer {
	return ServiceTokenIssuer{Check: oidc.ServiceClientCheck{}.FromJson(document), Signer: oidc.TokenSignerFactory{}.FromJson(document)}
}

func (factory SessionsFactory) ConfiguredServiceTokens() ServiceTokenIssuer {
	return factory.ServiceTokens(os.Getenv(factory.variable()))
}

func (factory SessionsFactory) realm(document string) RealmSessions {
	settings, _ := oidc.SettingsReader{}.FromJson(document)
	signer := oidc.TokenSignerFactory{}.FromJson(document)
	verifier := oidc.ExactIssuerVerifier{Issuer: settings.Issuer, Jwks: settings.Jwks, Keys: SignerKeys{Signer: signer}, Rules: oidc.MesailClaimRules{Audience: settings.Audience}}
	people := DirectoryPeople{Client: transport.SingleUseTransport{}.Client(factory.timeout()), BaseUrl: factory.loginSettings(document).Directory.Url, Signer: signer}
	logins := Logins{Realm: oidc.RealmSessionFactory{}.FromJson(document), People: people, Signer: signer}
	return RealmSessions{Logins: logins, Account: oidc.RealmAccountFactory{}.FromJson(document), Gate: oidc.NewGate(verifier, time.Now)}
}

func (SessionsFactory) loginSettings(document string) LoginSettings {
	snapshot := LoginSnapshot{}
	_ = json.Unmarshal([]byte(document), &snapshot)
	return snapshot.Yon
}

func (SessionsFactory) timeout() time.Duration {
	return 10 * time.Second
}

func (SessionsFactory) variable() string {
	return "YON_APPLICATION_JSON"
}
