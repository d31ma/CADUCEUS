package oidc

import (
	"cmp"
	"encoding/json"
	"os"
	"time"
)

type RealmSessionFactory struct{}

func (factory RealmSessionFactory) Configured() RealmSession {
	return factory.FromJson(os.Getenv(SettingsReader{}.variable()))
}

func (factory RealmSessionFactory) FromJson(document string) RealmSession {
	snapshot := ApplicationSnapshot{}
	_ = json.Unmarshal([]byte(document), &snapshot)
	sessions := SessionIssuerFactory{}.FromJson(document)
	sessions.Problem = cmp.Or(sessions.Problem, factory.complete(snapshot.Yon.Oidc))
	return RealmSession{Sessions: sessions, Verifier: VerifierFactory{}.Keycloak(snapshot.Yon.Oidc.Keycloak), Now: time.Now}
}

func (RealmSessionFactory) complete(settings Settings) error {
	if (IssuerPattern{Pattern: settings.Keycloak}).HasPlaceholder() || (IssuerPattern{Pattern: settings.Admin}).HasPlaceholder() {
		return NotConfigured{}
	}
	return nil
}
