package oidc

import (
	"errors"
	"testing"
)

func TestSessionIssuerFactoryNeedsTheSessionKey(t *testing.T) {
	err := SessionIssuerFactory{}.FromJson(`{"yon":{"oidc":{"issuer":"https://x/api/v1/identity","jwks":"https://x/api/v1/identity/jwks","audience":"mesail-api","keycloak":"https://x/realms/mesail/protocol/openid-connect"},"identity":{"client":{"secret":"s"}}}}`).Problem
	if !errors.Is(err, NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAnUnconfiguredRealmSessionIsUnavailable(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", "{}")
	session := RealmSessionFactory{}.Configured()
	_, signInErr := session.SignIn("ana", "right", "123456")
	_, refreshErr := session.Refresh("refresh")
	signOutErr := session.SignOut("refresh")
	if !errors.Is(signInErr, SessionUnavailable{}) || !errors.Is(refreshErr, SessionUnavailable{}) || !errors.Is(signOutErr, SessionUnavailable{}) {
		t.Fatalf("got %v, %v, %v", signInErr, refreshErr, signOutErr)
	}
}

func TestSessionErrorsTreatsAMissingConfigurationAsUnavailable(t *testing.T) {
	if err := (SessionErrors{}).Normalized(NotConfigured{}); !errors.Is(err, SessionUnavailable{}) {
		t.Fatalf("got %v", err)
	}
}
