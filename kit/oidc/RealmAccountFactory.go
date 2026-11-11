package oidc

import (
	"cmp"
	"encoding/json"
	"os"
	"time"
)

type RealmAccountFactory struct{}

func (factory RealmAccountFactory) Configured() RealmAccount {
	return factory.FromJson(os.Getenv(SettingsReader{}.variable()))
}

func (factory RealmAccountFactory) FromJson(document string) RealmAccount {
	snapshot := ApplicationSnapshot{}
	_ = json.Unmarshal([]byte(document), &snapshot)
	session := RealmSessionFactory{}.FromJson(document)
	session.Sessions.Problem = cmp.Or(session.Sessions.Problem, AccountManagerFactory{}.complete(snapshot.Yon))
	return RealmAccount{Session: session, Now: time.Now}
}
