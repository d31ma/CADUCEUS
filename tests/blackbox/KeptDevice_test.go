//go:build blackbox

package blackbox

import (
	"net/http"
	"testing"
)

func changedPassword(t *testing.T, stack Stack, person newPerson) newPerson {
	t.Helper()
	first := signIn(t, stack, person.Username, person.Password)
	newPassword := person.Password + "-Own"
	change := map[string]string{"currentPassword": person.Password, "newPassword": newPassword}
	stack.Call(t, http.MethodPost, "/api/v1/identity/password", first.AccessToken, change).Expect(t, http.StatusOK)
	return newPerson{Id: person.Id, Username: person.Username, Password: newPassword}
}

func TestAcOf1KeptSignInSaysKept(t *testing.T) {
	stack := NewStack(t)
	password := stack.Password(t, "ANA")
	kept := keptSignIn(t, stack, "ana", password, true)
	plain := keptSignIn(t, stack, "ana", password, false)
	if !kept.Kept || plain.Kept {
		t.Fatalf("want kept only when asked, got %v and %v", kept.Kept, plain.Kept)
	}
}

func TestAcOf1RefreshAndSwitchKeepTheKeptFlag(t *testing.T) {
	stack := NewStack(t)
	kept := keptSignIn(t, stack, "ana", stack.Password(t, "ANA"), true)
	var refreshed session
	refreshReply(t, stack, kept.SessionKey).Expect(t, http.StatusOK).Decode(t, &refreshed)
	switched := switchDomain(t, stack, refreshed.SessionKey, "example.org")
	if !refreshed.Kept || !switched.Kept {
		t.Fatalf("want kept after refresh and switch, got %v and %v", refreshed.Kept, switched.Kept)
	}
}

func TestAcOf9SignOutOnAllDevicesRevokesEveryKeptSession(t *testing.T) {
	stack := NewStack(t)
	dan := changedPassword(t, stack, createPerson(t, stack, fixture(t, stack, "ANA").AccessToken))
	laptop := keptSignIn(t, stack, dan.Username, dan.Password, true)
	phone := keptSignIn(t, stack, dan.Username, dan.Password, true)
	stack.Call(t, http.MethodDelete, "/api/v1/identity/sessions", laptop.AccessToken, nil).Expect(t, http.StatusNoContent)
	refreshReply(t, stack, phone.SessionKey).ExpectError(t, http.StatusForbidden, "access_revoked")
	refreshReply(t, stack, laptop.SessionKey).ExpectError(t, http.StatusForbidden, "access_revoked")
	keptSignIn(t, stack, dan.Username, dan.Password, true)
}

func TestAcOf9OwnPasswordChangeRevokesTheOtherSessions(t *testing.T) {
	stack := NewStack(t)
	dan := changedPassword(t, stack, createPerson(t, stack, fixture(t, stack, "ANA").AccessToken))
	laptop := keptSignIn(t, stack, dan.Username, dan.Password, true)
	phone := keptSignIn(t, stack, dan.Username, dan.Password, true)
	change := map[string]string{"currentPassword": dan.Password, "newPassword": dan.Password + "-Again"}
	var fresh session
	stack.Call(t, http.MethodPost, "/api/v1/identity/password", laptop.AccessToken, change).Expect(t, http.StatusOK).Decode(t, &fresh)
	refreshReply(t, stack, phone.SessionKey).ExpectError(t, http.StatusForbidden, "access_revoked")
	refreshReply(t, stack, fresh.SessionKey).Expect(t, http.StatusOK)
}

func TestAcOf9AdminPasswordResetRevokesTheKeptSessions(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	dan := changedPassword(t, stack, createPerson(t, stack, ana.AccessToken))
	phone := keptSignIn(t, stack, dan.Username, dan.Password, true)
	resetPasswordReply(t, stack, ana.AccessToken, dan.Id, dan.Password+"-Reset").Expect(t, http.StatusNoContent)
	refreshReply(t, stack, phone.SessionKey).ExpectError(t, http.StatusForbidden, "access_revoked")
}

func TestAcOf9SignOutOfOneDeviceKeepsTheOthers(t *testing.T) {
	stack := NewStack(t)
	dan := changedPassword(t, stack, createPerson(t, stack, fixture(t, stack, "ANA").AccessToken))
	laptop := keptSignIn(t, stack, dan.Username, dan.Password, true)
	phone := keptSignIn(t, stack, dan.Username, dan.Password, true)
	request := map[string]string{"sessionKey": laptop.SessionKey}
	stack.Call(t, http.MethodDelete, "/api/v1/identity/session", "", request).Expect(t, http.StatusNoContent)
	refreshReply(t, stack, laptop.SessionKey).Expect(t, http.StatusUnauthorized)
	refreshReply(t, stack, phone.SessionKey).Expect(t, http.StatusOK)
}
