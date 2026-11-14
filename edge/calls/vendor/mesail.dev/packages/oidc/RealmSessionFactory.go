package oidc

import (
	"encoding/json"
	"os"
	"time"
)

type RealmSessionFactory struct{}

func (factory RealmSessionFactory) Configured() RealmSession {
	return factory.FromJson(os.Getenv(SettingsReader{}.variable()))
}

func (RealmSessionFactory) FromJson(document string) RealmSession {
	snapshot := ApplicationSnapshot{}
	_ = json.Unmarshal([]byte(document), &snapshot)
	sessions := SessionIssuerFactory{}.FromJson(document)
	return RealmSession{Sessions: sessions, Verifier: VerifierFactory{}.Keycloak(snapshot.Yon.Oidc.Keycloak), Now: time.Now}
}
