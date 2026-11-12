package keycloak

// tachyon: example secrets

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"
)

func usersPath(rest string) string {
	return "/idp/admin/realms/mesail/users" + rest
}

func decodedBody(t *testing.T, request recordedRequest) map[string]any {
	body := map[string]any{}
	err := json.Unmarshal([]byte(request.body), &body)
	if err != nil {
		t.Fatalf("the body %q is not a JSON object: %v", request.body, err)
	}
	return body
}

func expectRequests(t *testing.T, fake *fakeKeycloak, expected []string) {
	actual := []string{}
	for _, request := range fake.adminRequests() {
		actual = append(actual, request.method+" "+request.uri)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("got requests %q, expected %q", actual, expected)
	}
}

func expectRefused(t *testing.T, err error, status int, action string) {
	refused := AdminRefused{}
	if !errors.As(err, &refused) || refused.Status != status || refused.Action != action {
		t.Fatalf("got %v", err)
	}
}

func expectMissing(t *testing.T, err error) {
	if !errors.As(err, &UserMissing{}) {
		t.Fatalf("got %v", err)
	}
}

func expectPasswordRefused(t *testing.T, err error, message string) {
	refused := PasswordRefused{}
	if !errors.As(err, &refused) || refused.Message != message {
		t.Fatalf("got %v", err)
	}
}

func expectPlainFailure(t *testing.T, err error) {
	if err == nil || errors.As(err, &AdminRefused{}) || errors.As(err, &UserMissing{}) || errors.As(err, &PasswordRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestCreatePostsTheUserAndReturnsTheIdFromLocation(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodPost, usersPath(""), fakeAnswer{status: http.StatusCreated, location: fake.server.URL + usersPath("/9f1c-42")})
	id, err := fake.users().Create(NewUser{Username: "ada", Password: "example-password"})
	if err != nil || id != "9f1c-42" {
		t.Fatalf("got %q and %v", id, err)
	}
	expected := map[string]any{
		"username":    "ada",
		"enabled":     true,
		"attributes":  map[string]any{"mesail.mustChangePassword": []any{"true"}},
		"credentials": []any{map[string]any{"type": "password", "value": "example-password", "temporary": false}},
	}
	body := decodedBody(t, fake.adminRequests()[0])
	if !reflect.DeepEqual(body, expected) {
		t.Fatalf("got body %v", body)
	}
}

func TestCreateWithoutLocationSearchesForTheId(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodPost, usersPath(""), fakeAnswer{status: http.StatusCreated})
	fake.on(http.MethodGet, usersPath("?exact=true&username=ada"), fakeAnswer{status: http.StatusOK, body: `[{"id":"u7","username":"ada"}]`})
	id, err := fake.users().Create(NewUser{Username: "ada", Password: "example-password"})
	if err != nil || id != "u7" {
		t.Fatalf("got %q and %v", id, err)
	}
}

func TestCreateReportsAnExistingUser(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodPost, usersPath(""), fakeAnswer{status: http.StatusConflict})
	_, err := fake.users().Create(NewUser{Username: "ada", Password: "example-password"})
	exists := UserExists{}
	if !errors.As(err, &exists) || exists.Username != "ada" {
		t.Fatalf("got %v", err)
	}
}

func TestCreateReportsTheErrorMessageOfARefusedPassword(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodPost, usersPath(""), fakeAnswer{status: http.StatusBadRequest, body: `{"errorMessage":"invalidPasswordMinLengthMessage"}`})
	_, err := fake.users().Create(NewUser{Username: "ada", Password: "short"})
	expectPasswordRefused(t, err, "invalidPasswordMinLengthMessage")
}

func TestCreateReportsTheErrorDescriptionOfARefusedPassword(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodPost, usersPath(""), fakeAnswer{status: http.StatusBadRequest, body: `{"error_description":"Password policy not met"}`})
	_, err := fake.users().Create(NewUser{Username: "ada", Password: "short"})
	expectPasswordRefused(t, err, "Password policy not met")
}

func TestCreateReportsARefusedPasswordWithoutAJsonBody(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodPost, usersPath(""), fakeAnswer{status: http.StatusBadRequest, body: `bad request`})
	_, err := fake.users().Create(NewUser{Username: "ada", Password: "short"})
	expectPasswordRefused(t, err, "")
}

func TestCreateReportsOtherStatusesAsRefused(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodPost, usersPath(""), fakeAnswer{status: http.StatusForbidden})
	_, err := fake.users().Create(NewUser{Username: "ada", Password: "example-password"})
	expectRefused(t, err, http.StatusForbidden, "create the user")
}

func TestCreateReportsAServerErrorAsUnavailable(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodPost, usersPath(""), fakeAnswer{status: http.StatusBadGateway})
	_, err := fake.users().Create(NewUser{Username: "ada", Password: "example-password"})
	if !errors.As(err, &Unavailable{}) || errors.As(err, &AdminRefused{}) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateFailsWithoutSettings(t *testing.T) {
	_, err := NewUsers(UserSettings{}, http.DefaultClient).Create(NewUser{Username: "ada", Password: "example-password"})
	if !errors.As(err, &NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestIdOfFindsTheExactUserWhateverItsCase(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("?exact=true&username=Ada+L%2Bx%40example.com"), fakeAnswer{status: http.StatusOK, body: `[{"id":"u1","username":"ada l+x@example.com"},{"id":"u2","username":"adam"}]`})
	id, err := fake.users().IdOf("Ada L+x@example.com")
	if err != nil || id != "u1" {
		t.Fatalf("got %q and %v", id, err)
	}
}

func TestIdOfReportsAMissingUser(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("?exact=true&username=ada"), fakeAnswer{status: http.StatusOK, body: `[{"id":"u2","username":"adam"}]`})
	_, err := fake.users().IdOf("ada")
	expectMissing(t, err)
}

func TestIdOfReportsTwoMatchesAsMissing(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("?exact=true&username=ada"), fakeAnswer{status: http.StatusOK, body: `[{"id":"u1","username":"ada"},{"id":"u2","username":"ADA"}]`})
	_, err := fake.users().IdOf("ada")
	expectMissing(t, err)
}

func TestIdOfReportsARefusedSearch(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("?exact=true&username=ada"), fakeAnswer{status: http.StatusForbidden})
	_, err := fake.users().IdOf("ada")
	expectRefused(t, err, http.StatusForbidden, "search for the user")
}

func TestIdOfFailsOnAnAnswerThatIsNotJson(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("?exact=true&username=ada"), fakeAnswer{status: http.StatusOK, body: `{`})
	_, err := fake.users().IdOf("ada")
	expectPlainFailure(t, err)
}

func TestSetEnabledWritesBackTheWholeUser(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u%2F1"), fakeAnswer{status: http.StatusOK, body: `{"id":"u/1","username":"ada","enabled":true,"totp":false}`})
	fake.on(http.MethodPut, usersPath("/u%2F1"), fakeAnswer{status: http.StatusNoContent})
	err := fake.users().SetEnabled("u/1", false)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]any{"id": "u/1", "username": "ada", "enabled": false, "totp": false}
	body := decodedBody(t, fake.adminRequests()[1])
	if !reflect.DeepEqual(body, expected) {
		t.Fatalf("got body %v", body)
	}
}

func TestSetEnabledReportsAMissingUserOnRead(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1"), fakeAnswer{status: http.StatusNotFound})
	err := fake.users().SetEnabled("u1", true)
	expectMissing(t, err)
}

func TestSetEnabledReportsAMissingUserOnWrite(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1"), fakeAnswer{status: http.StatusOK, body: `{"id":"u1"}`})
	fake.on(http.MethodPut, usersPath("/u1"), fakeAnswer{status: http.StatusNotFound})
	err := fake.users().SetEnabled("u1", true)
	expectMissing(t, err)
}

func TestSetEnabledFailsOnAUserThatIsNotJson(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1"), fakeAnswer{status: http.StatusOK, body: `[]`})
	err := fake.users().SetEnabled("u1", true)
	expectPlainFailure(t, err)
}

func TestSetEnabledFailsWithoutSettings(t *testing.T) {
	err := NewUsers(UserSettings{}, http.DefaultClient).SetEnabled("u1", true)
	if !errors.As(err, &NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSetEnabledFailsWhenTheWriteCannotBeSent(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.limit = 1
	fake.on(http.MethodGet, usersPath("/u1"), fakeAnswer{status: http.StatusOK, body: `{"id":"u1"}`})
	err := fake.users().SetEnabled("u1", true)
	expectRefused(t, err, http.StatusUnauthorized, "issue an admin access token")
}

func resetPasswordFake(t *testing.T, user string) *fakeKeycloak {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodPut, usersPath("/u1/reset-password"), fakeAnswer{status: http.StatusNoContent})
	fake.on(http.MethodGet, usersPath("/u1"), fakeAnswer{status: http.StatusOK, body: user})
	fake.on(http.MethodPut, usersPath("/u1"), fakeAnswer{status: http.StatusNoContent})
	fake.on(http.MethodPost, usersPath("/u1/logout"), fakeAnswer{status: http.StatusNoContent})
	return fake
}

func TestResetPasswordSetsThePasswordMarksItAndSignsTheUserOut(t *testing.T) {
	fake := resetPasswordFake(t, `{"id":"u1","attributes":{"locale":["en"]}}`)
	err := fake.users().ResetPassword("u1", "example-password")
	if err != nil {
		t.Fatal(err)
	}
	expectRequests(t, fake, []string{"PUT " + usersPath("/u1/reset-password"), "GET " + usersPath("/u1"), "PUT " + usersPath("/u1"), "POST " + usersPath("/u1/logout")})
	credential := decodedBody(t, fake.adminRequests()[0])
	if !reflect.DeepEqual(credential, map[string]any{"type": "password", "value": "example-password", "temporary": false}) {
		t.Fatalf("got credential %v", credential)
	}
	user := decodedBody(t, fake.adminRequests()[2])
	attributes := map[string]any{"locale": []any{"en"}, "mesail.mustChangePassword": []any{"true"}}
	if !reflect.DeepEqual(user, map[string]any{"id": "u1", "attributes": attributes}) {
		t.Fatalf("got user %v", user)
	}
}

func TestResetPasswordAddsAttributesWhenTheUserHasNone(t *testing.T) {
	fake := resetPasswordFake(t, `{"id":"u1"}`)
	err := fake.users().ResetPassword("u1", "example-password")
	user := decodedBody(t, fake.adminRequests()[2])
	if err != nil || !reflect.DeepEqual(user["attributes"], map[string]any{"mesail.mustChangePassword": []any{"true"}}) {
		t.Fatalf("got user %v and %v", user, err)
	}
}

func TestResetPasswordAddsAttributesWhenTheyAreNull(t *testing.T) {
	fake := resetPasswordFake(t, `{"id":"u1","attributes":null}`)
	err := fake.users().ResetPassword("u1", "example-password")
	user := decodedBody(t, fake.adminRequests()[2])
	if err != nil || !reflect.DeepEqual(user["attributes"], map[string]any{"mesail.mustChangePassword": []any{"true"}}) {
		t.Fatalf("got user %v and %v", user, err)
	}
}

func TestResetPasswordReportsARefusedPassword(t *testing.T) {
	fake := resetPasswordFake(t, `{"id":"u1"}`)
	fake.on(http.MethodPut, usersPath("/u1/reset-password"), fakeAnswer{status: http.StatusBadRequest, body: `{"error_description":"Password policy not met"}`})
	err := fake.users().ResetPassword("u1", "short")
	expectPasswordRefused(t, err, "Password policy not met")
	expectRequests(t, fake, []string{"PUT " + usersPath("/u1/reset-password")})
}

func TestResetPasswordReportsAMissingUser(t *testing.T) {
	fake := resetPasswordFake(t, `{"id":"u1"}`)
	fake.on(http.MethodPut, usersPath("/u1/reset-password"), fakeAnswer{status: http.StatusNotFound})
	err := fake.users().ResetPassword("u1", "example-password")
	expectMissing(t, err)
}

func TestResetPasswordReportsOtherStatusesAsRefused(t *testing.T) {
	fake := resetPasswordFake(t, `{"id":"u1"}`)
	fake.on(http.MethodPut, usersPath("/u1/reset-password"), fakeAnswer{status: http.StatusForbidden})
	err := fake.users().ResetPassword("u1", "example-password")
	expectRefused(t, err, http.StatusForbidden, "reset the password")
}

func TestResetPasswordFailsWithoutSettings(t *testing.T) {
	err := NewUsers(UserSettings{}, http.DefaultClient).ResetPassword("u1", "example-password")
	if !errors.As(err, &NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestResetPasswordReportsAUserThatVanishesBeforeTheMark(t *testing.T) {
	fake := resetPasswordFake(t, `{"id":"u1"}`)
	fake.on(http.MethodGet, usersPath("/u1"), fakeAnswer{status: http.StatusNotFound})
	err := fake.users().ResetPassword("u1", "example-password")
	expectMissing(t, err)
}

func TestResetPasswordFailsOnAttributesThatAreNotAnObject(t *testing.T) {
	fake := resetPasswordFake(t, `{"id":"u1","attributes":"locale"}`)
	err := fake.users().ResetPassword("u1", "example-password")
	expectPlainFailure(t, err)
}

func TestResetPasswordReportsARefusedMark(t *testing.T) {
	fake := resetPasswordFake(t, `{"id":"u1"}`)
	fake.on(http.MethodPut, usersPath("/u1"), fakeAnswer{status: http.StatusForbidden})
	err := fake.users().ResetPassword("u1", "example-password")
	expectRefused(t, err, http.StatusForbidden, "update the user")
}

func TestResetPasswordReportsARefusedLogout(t *testing.T) {
	fake := resetPasswordFake(t, `{"id":"u1"}`)
	fake.on(http.MethodPost, usersPath("/u1/logout"), fakeAnswer{status: http.StatusForbidden})
	err := fake.users().ResetPassword("u1", "example-password")
	expectRefused(t, err, http.StatusForbidden, "sign the user out")
}

func TestResetPasswordFailsWhenTheLogoutCannotBeSent(t *testing.T) {
	fake := resetPasswordFake(t, `{"id":"u1"}`)
	fake.limit = 3
	err := fake.users().ResetPassword("u1", "example-password")
	expectRefused(t, err, http.StatusUnauthorized, "issue an admin access token")
}

func TestRemoveTotpDeletesOnlyTheOneTimePasswordCredentials(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1/credentials"), fakeAnswer{status: http.StatusOK, body: `[{"id":"c1","type":"password"},{"id":"c2","type":"otp"},{"id":"c3","type":"otp"}]`})
	fake.on(http.MethodDelete, usersPath("/u1/credentials/c2"), fakeAnswer{status: http.StatusNoContent})
	fake.on(http.MethodDelete, usersPath("/u1/credentials/c3"), fakeAnswer{status: http.StatusNoContent})
	err := fake.users().RemoveTotp("u1")
	if err != nil {
		t.Fatal(err)
	}
	expectRequests(t, fake, []string{"GET " + usersPath("/u1/credentials"), "DELETE " + usersPath("/u1/credentials/c2"), "DELETE " + usersPath("/u1/credentials/c3")})
}

func TestRemoveTotpWithoutOneTimePasswordsDoesNothing(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1/credentials"), fakeAnswer{status: http.StatusOK, body: `[{"id":"c1","type":"password"}]`})
	err := fake.users().RemoveTotp("u1")
	if err != nil {
		t.Fatal(err)
	}
	expectRequests(t, fake, []string{"GET " + usersPath("/u1/credentials")})
}

func TestRemoveTotpReportsAMissingUser(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1/credentials"), fakeAnswer{status: http.StatusNotFound})
	err := fake.users().RemoveTotp("u1")
	expectMissing(t, err)
}

func TestRemoveTotpFailsOnAnAnswerThatIsNotJson(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1/credentials"), fakeAnswer{status: http.StatusOK, body: `{}`})
	err := fake.users().RemoveTotp("u1")
	expectPlainFailure(t, err)
}

func TestRemoveTotpReportsARefusedDelete(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1/credentials"), fakeAnswer{status: http.StatusOK, body: `[{"id":"c2","type":"otp"}]`})
	fake.on(http.MethodDelete, usersPath("/u1/credentials/c2"), fakeAnswer{status: http.StatusForbidden})
	err := fake.users().RemoveTotp("u1")
	expectRefused(t, err, http.StatusForbidden, "remove the user credential")
}

func TestRemoveTotpFailsWhenTheDeleteCannotBeSent(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.limit = 1
	fake.on(http.MethodGet, usersPath("/u1/credentials"), fakeAnswer{status: http.StatusOK, body: `[{"id":"c2","type":"otp"}]`})
	err := fake.users().RemoveTotp("u1")
	expectRefused(t, err, http.StatusUnauthorized, "issue an admin access token")
}

func TestRemoveTotpFailsWithoutSettings(t *testing.T) {
	err := NewUsers(UserSettings{}, http.DefaultClient).RemoveTotp("u1")
	if !errors.As(err, &NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func passkeysPath(userId string) string {
	return "/idp/admin/realms/mesail/mesail/users/" + userId + "/passkeys"
}

func removePasskeysFake(t *testing.T, status int) *fakeKeycloak {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodDelete, passkeysPath("u1"), fakeAnswer{status: status})
	fake.on(http.MethodPost, usersPath("/u1/logout"), fakeAnswer{status: http.StatusNoContent})
	return fake
}

func TestRemovePasskeysDeletesThemAllAndSignsTheUserOut(t *testing.T) {
	fake := removePasskeysFake(t, http.StatusNoContent)
	err := fake.users().RemovePasskeys("u1")
	if err != nil {
		t.Fatal(err)
	}
	expectRequests(t, fake, []string{"DELETE " + passkeysPath("u1"), "POST " + usersPath("/u1/logout")})
}

func TestRemovePasskeysEscapesTheUserId(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodDelete, passkeysPath("a%2Fb"), fakeAnswer{status: http.StatusNoContent})
	fake.on(http.MethodPost, usersPath("/a%2Fb/logout"), fakeAnswer{status: http.StatusNoContent})
	err := fake.users().RemovePasskeys("a/b")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRemovePasskeysReportsAMissingUserWithoutASignOut(t *testing.T) {
	fake := removePasskeysFake(t, http.StatusNotFound)
	err := fake.users().RemovePasskeys("u1")
	expectMissing(t, err)
	expectRequests(t, fake, []string{"DELETE " + passkeysPath("u1")})
}

func TestRemovePasskeysReportsARefusedDelete(t *testing.T) {
	fake := removePasskeysFake(t, http.StatusForbidden)
	err := fake.users().RemovePasskeys("u1")
	expectRefused(t, err, http.StatusForbidden, "remove the user passkeys")
}

func TestRemovePasskeysReportsARefusedLogout(t *testing.T) {
	fake := removePasskeysFake(t, http.StatusNoContent)
	fake.on(http.MethodPost, usersPath("/u1/logout"), fakeAnswer{status: http.StatusForbidden})
	err := fake.users().RemovePasskeys("u1")
	expectRefused(t, err, http.StatusForbidden, "sign the user out")
}

func TestRemovePasskeysFailsWhenTheLogoutCannotBeSent(t *testing.T) {
	fake := removePasskeysFake(t, http.StatusNoContent)
	fake.limit = 1
	err := fake.users().RemovePasskeys("u1")
	expectRefused(t, err, http.StatusUnauthorized, "issue an admin access token")
}

func TestRemovePasskeysFailsWithoutSettings(t *testing.T) {
	err := NewUsers(UserSettings{}, http.DefaultClient).RemovePasskeys("u1")
	if !errors.As(err, &NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteRemovesTheUser(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodDelete, usersPath("/u1"), fakeAnswer{status: http.StatusNoContent})
	err := fake.users().Delete("u1")
	if err != nil {
		t.Fatal(err)
	}
	expectRequests(t, fake, []string{"DELETE " + usersPath("/u1")})
}

func TestDeleteOfAMissingUserSucceeds(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodDelete, usersPath("/u1"), fakeAnswer{status: http.StatusNotFound})
	err := fake.users().Delete("u1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteReportsOtherStatusesAsRefused(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodDelete, usersPath("/u1"), fakeAnswer{status: http.StatusForbidden})
	err := fake.users().Delete("u1")
	expectRefused(t, err, http.StatusForbidden, "delete the user")
}

func TestIsOperatorFindsTheOperatorRole(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1/role-mappings/realm/composite"), fakeAnswer{status: http.StatusOK, body: `[{"name":"default-roles-mesail"},{"name":"mesail-operator"}]`})
	operator, err := fake.users().IsOperator("u1")
	if err != nil || !operator {
		t.Fatalf("got %v and %v", operator, err)
	}
}

func TestIsOperatorIsFalseWithoutTheRole(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1/role-mappings/realm/composite"), fakeAnswer{status: http.StatusOK, body: `[{"name":"default-roles-mesail"}]`})
	operator, err := fake.users().IsOperator("u1")
	if err != nil || operator {
		t.Fatalf("got %v and %v", operator, err)
	}
}

func TestIsOperatorReportsAMissingUser(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1/role-mappings/realm/composite"), fakeAnswer{status: http.StatusNotFound})
	_, err := fake.users().IsOperator("u1")
	expectMissing(t, err)
}

func TestIsOperatorFailsOnAnAnswerThatIsNotJson(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("/u1/role-mappings/realm/composite"), fakeAnswer{status: http.StatusOK, body: `"admin"`})
	_, err := fake.users().IsOperator("u1")
	expectPlainFailure(t, err)
}

func TestIsOperatorFailsWithoutSettings(t *testing.T) {
	_, err := NewUsers(UserSettings{}, http.DefaultClient).IsOperator("u1")
	if !errors.As(err, &NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}
