package keycloak

// tachyon: example secrets

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func expectNotConfigured(t *testing.T, settings UserSettings) {
	_, err := NewUsers(settings, http.DefaultClient).IdOf("someone")
	if !errors.As(err, &NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAMissingBaseUrlIsNotConfigured(t *testing.T) {
	expectNotConfigured(t, UserSettings{ClientId: "mesail-directory", ClientSecret: "example-secret"})
}

func TestAMissingClientIdIsNotConfigured(t *testing.T) {
	expectNotConfigured(t, UserSettings{BaseUrl: "http://keycloak:8080/idp", ClientSecret: "example-secret"})
}

func TestAMissingClientSecretIsNotConfigured(t *testing.T) {
	expectNotConfigured(t, UserSettings{BaseUrl: "http://keycloak:8080/idp", ClientId: "mesail-directory"})
}

func TestTheTokenComesFromClientCredentialsInTheMesailRealm(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodDelete, "/idp/admin/realms/mesail/users/u1", fakeAnswer{status: http.StatusNoContent})
	err := fake.users().Delete("u1")
	if err != nil {
		t.Fatal(err)
	}
	token := fake.recorded()[0]
	form, _ := url.ParseQuery(token.body)
	if token.method != http.MethodPost || token.contentType != "application/x-www-form-urlencoded" {
		t.Fatalf("got %s with %q", token.method, token.contentType)
	}
	if form.Get("grant_type") != "client_credentials" || form.Get("client_id") != "mesail-directory" || form.Get("client_secret") != "example-secret" {
		t.Fatalf("got form %v", form)
	}
}

func TestAdminRequestsCarryTheBearerTokenAndJsonType(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodDelete, "/idp/admin/realms/mesail/users/u1", fakeAnswer{status: http.StatusNoContent})
	_ = fake.users().Delete("u1")
	admin := fake.adminRequests()[0]
	if admin.authorization != "Bearer example-token" || admin.contentType != "application/json" {
		t.Fatalf("got %q and %q", admin.authorization, admin.contentType)
	}
}

func TestARefusedTokenIsAdminRefused(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.token = fakeAnswer{status: http.StatusUnauthorized, body: `{}`}
	err := fake.users().Delete("u1")
	refused := AdminRefused{}
	if !errors.As(err, &refused) || refused.Status != http.StatusUnauthorized || refused.Action != "issue an admin access token" {
		t.Fatalf("got %v", err)
	}
}

func TestAnEmptyTokenIsAdminRefused(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.token = fakeAnswer{status: http.StatusOK, body: `{"access_token":""}`}
	err := fake.users().Delete("u1")
	refused := AdminRefused{}
	if !errors.As(err, &refused) || refused.Status != http.StatusOK {
		t.Fatalf("got %v", err)
	}
}

func TestATokenAnswerThatIsNotJsonFails(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.token = fakeAnswer{status: http.StatusOK, body: `not json`}
	err := fake.users().Delete("u1")
	if err == nil || errors.As(err, &AdminRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAServerErrorIsUnavailable(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.token = fakeAnswer{status: http.StatusServiceUnavailable}
	err := fake.users().Delete("u1")
	unavailable := Unavailable{}
	if !errors.As(err, &unavailable) || unavailable.Status != http.StatusServiceUnavailable || errors.As(err, &AdminRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestANetworkFailureIsNoneOfTheTypedErrors(t *testing.T) {
	fake := newFakeKeycloak(t)
	users := fake.users()
	fake.server.Close()
	err := users.Delete("u1")
	if err == nil || errors.As(err, &AdminRefused{}) || errors.As(err, &Unavailable{}) || errors.As(err, &UserMissing{}) {
		t.Fatalf("got %v", err)
	}
}

func TestABaseUrlThatIsNotAUrlFails(t *testing.T) {
	users := NewUsers(UserSettings{BaseUrl: "http://keycloak\x7f", ClientId: "mesail-directory", ClientSecret: "example-secret"}, http.DefaultClient)
	err := users.Delete("u1")
	if err == nil || errors.As(err, &NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAnAdminRequestThatCannotBeBuiltFails(t *testing.T) {
	fake := newFakeKeycloak(t)
	_, err := fake.users().admin.read("BAD METHOD", "/users")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestABodyThatCannotBeEncodedFails(t *testing.T) {
	fake := newFakeKeycloak(t)
	_, err := fake.users().admin.write(http.MethodPut, "/users", make(chan int))
	if err == nil || len(fake.recorded()) != 0 {
		t.Fatalf("got %v after %d requests", err, len(fake.recorded()))
	}
}

func expectMessage(t *testing.T, err error, expected string) {
	if err.Error() != expected {
		t.Fatalf("got %q, expected %q", err.Error(), expected)
	}
}

func TestErrorMessagesNameTheProblem(t *testing.T) {
	expectMessage(t, NotConfigured{}, "the Keycloak user client is not configured: the base URL, client id and client secret are required")
	expectMessage(t, AdminRefused{Status: http.StatusForbidden, Action: "delete the user"}, "Keycloak refused to delete the user (HTTP status 403)")
	expectMessage(t, PasswordRefused{Message: "invalidPasswordMinLengthMessage"}, "Keycloak refused the password: invalidPasswordMinLengthMessage")
	expectMessage(t, UserExists{Username: "ada"}, `a Keycloak user named "ada" already exists`)
	expectMessage(t, UserMissing{}, "the Keycloak user does not exist")
	expectMessage(t, Unavailable{Status: http.StatusBadGateway}, "Keycloak is unavailable (HTTP status 502)")
}
