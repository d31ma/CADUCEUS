package oidc

import (
	"crypto/subtle"
	"encoding/json"
	"os"
)

type ServiceClientCheck struct {
	Client ClientSetting
}

func (check ServiceClientCheck) Configured() ServiceClientCheck {
	return check.FromJson(os.Getenv(SettingsReader{}.variable()))
}

func (ServiceClientCheck) FromJson(document string) ServiceClientCheck {
	snapshot := ApplicationSnapshot{}
	_ = json.Unmarshal([]byte(document), &snapshot)
	return ServiceClientCheck{Client: snapshot.Yon.Service.Client}
}

func (check ServiceClientCheck) Accepts(clientId string, secret string) bool {
	known := clientId == AuthorizedParties{}.Service()
	current := check.matches(check.Client.Secret, secret)
	previous := check.matches(check.Client.Previous, secret)
	return check.allowed(known, current, previous)
}

func (ServiceClientCheck) matches(expected string, given string) bool {
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(given)) == 1
}

func (ServiceClientCheck) allowed(known bool, current bool, previous bool) bool {
	return known && (current || previous)
}
