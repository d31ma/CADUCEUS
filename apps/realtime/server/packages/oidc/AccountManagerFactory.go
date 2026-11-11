package oidc

import (
	"cmp"
	"encoding/json"
	"os"
	"time"
)

type AccountManagerFactory struct{}

func (factory AccountManagerFactory) Configured() AccountManager {
	return factory.FromJson(os.Getenv(SettingsReader{}.variable()))
}

func (factory AccountManagerFactory) FromJson(document string) AccountManager {
	snapshot := ApplicationSnapshot{}
	_ = json.Unmarshal([]byte(document), &snapshot)
	sessions := SessionIssuerFactory{}.FromJson(document)
	sessions.Problem = cmp.Or(sessions.Problem, factory.complete(snapshot.Yon))
	return AccountManager{Sessions: sessions, Admin: snapshot.Yon.Oidc.Admin, Now: time.Now}
}

func (AccountManagerFactory) complete(settings ApplicationSettings) error {
	if settings.Oidc.Admin == "" || settings.Identity.Account.Secret == "" {
		return NotConfigured{}
	}
	return nil
}
