package keycloak

// tachyon: example secrets

import (
	"net/http"
	"reflect"
	"testing"
)

func TestCreateTagsTheUserWithTheAddRequest(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodPost, usersPath(""), fakeAnswer{status: http.StatusCreated, location: fake.server.URL + usersPath("/u9")})
	_, err := fake.users().Create(NewUser{Username: "ada", Password: "example-password", AddRequest: "0f3a"})
	if err != nil {
		t.Fatalf("got %v", err)
	}
	expected := map[string]any{"mesail.mustChangePassword": []any{"true"}, "mesail.addRequest": []any{"0f3a"}}
	body := decodedBody(t, fake.adminRequests()[0])
	if !reflect.DeepEqual(body["attributes"], expected) {
		t.Fatalf("got attributes %v", body["attributes"])
	}
}

func TestTaggedReadsTheAddRequestOfTheUser(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("?exact=true&username=ada"), fakeAnswer{status: http.StatusOK, body: `[{"id":"u1","username":"ada"}]`})
	fake.on(http.MethodGet, usersPath("/u1"), fakeAnswer{status: http.StatusOK, body: `{"id":"u1","attributes":{"mesail.addRequest":["0f3a"],"mesail.mustChangePassword":["true"]}}`})
	user, err := fake.users().Tagged("ada")
	if err != nil || user != (TaggedUser{Id: "u1", AddRequest: "0f3a"}) {
		t.Fatalf("got %v and %v", user, err)
	}
}

func TestTaggedGivesAnEmptyTagForAUserWithoutOne(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("?exact=true&username=ada"), fakeAnswer{status: http.StatusOK, body: `[{"id":"u1","username":"ada"}]`})
	fake.on(http.MethodGet, usersPath("/u1"), fakeAnswer{status: http.StatusOK, body: `{"id":"u1","attributes":{"mesail.addRequest":[]}}`})
	user, err := fake.users().Tagged("ada")
	if err != nil || user != (TaggedUser{Id: "u1"}) {
		t.Fatalf("got %v and %v", user, err)
	}
}

func TestTaggedGivesAnEmptyTagForAUserWithoutAttributes(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("?exact=true&username=ada"), fakeAnswer{status: http.StatusOK, body: `[{"id":"u1","username":"ada"}]`})
	fake.on(http.MethodGet, usersPath("/u1"), fakeAnswer{status: http.StatusOK, body: `{"id":"u1"}`})
	user, err := fake.users().Tagged("ada")
	if err != nil || user != (TaggedUser{Id: "u1"}) {
		t.Fatalf("got %v and %v", user, err)
	}
}

func TestTaggedReportsAMissingUser(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("?exact=true&username=ada"), fakeAnswer{status: http.StatusOK, body: `[]`})
	_, err := fake.users().Tagged("ada")
	expectMissing(t, err)
}

func TestTaggedReportsAUserThatVanishesBeforeTheRead(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("?exact=true&username=ada"), fakeAnswer{status: http.StatusOK, body: `[{"id":"u1","username":"ada"}]`})
	fake.on(http.MethodGet, usersPath("/u1"), fakeAnswer{status: http.StatusNotFound})
	_, err := fake.users().Tagged("ada")
	expectMissing(t, err)
}

func TestTaggedFailsOnAttributesThatAreNotLists(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.on(http.MethodGet, usersPath("?exact=true&username=ada"), fakeAnswer{status: http.StatusOK, body: `[{"id":"u1","username":"ada"}]`})
	fake.on(http.MethodGet, usersPath("/u1"), fakeAnswer{status: http.StatusOK, body: `{"id":"u1","attributes":{"mesail.addRequest":"0f3a"}}`})
	_, err := fake.users().Tagged("ada")
	expectPlainFailure(t, err)
}
