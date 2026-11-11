// tachyon: example secrets
import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type L3MembershipEnrollTest struct {
	behaviors *BehaviorContext
}

func (test *L3MembershipEnrollTest) TestANewPersonGetsAnAccountAndAReservation() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	username := test.username()
	request := AddMemberRequest{Username: " " + strings.ToUpper(username) + " ", DisplayName: " Ana Example ", FirstPassword: "first-password"}
	person, existing, err := test.enroll().Enroll(request)
	reservation, _ := test.reservations().Current(username)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(!existing, "want a new person")
	Assert.Equals("kc-"+username, person.PersonId)
	Assert.Equals(username, person.Username)
	Assert.Equals("Ana Example", person.DisplayName)
	Assert.Equals("active", person.State)
	Assert.Equals("person", reservation.Owner)
	Assert.Equals("kc-"+username, reservation.PersonId)
	Assert.Equals("POST /users", calls())
}

func (test *L3MembershipEnrollTest) TestAKnownPersonIsFoundThroughTheReservation() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	username := test.known("active")
	person, existing, err := test.enroll().Enroll(AddMemberRequest{Username: username})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(existing, "want an existing person")
	Assert.Equals("kc-"+username, person.PersonId)
	Assert.Equals("Known Person", person.DisplayName)
	Assert.Equals("", calls())
}

func (test *L3MembershipEnrollTest) TestTheNameOfADeletedPersonIsNeverReused() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	username := test.known("deleted")
	_, _, err := test.enroll().Enroll(AddMemberRequest{Username: username, DisplayName: "Again", FirstPassword: "first-password"})
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *L3MembershipEnrollTest) TestAReservationWithoutAStoredPersonGivesAFreshPerson() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	username := test.username()
	test.reservations().ForPerson(username, "kc-"+username)
	person, existing, err := test.enroll().Enroll(AddMemberRequest{Username: username, DisplayName: "Cy Fresh"})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(!existing, "want a new person")
	Assert.Equals("kc-"+username, person.PersonId)
	Assert.Equals("Cy Fresh", person.DisplayName)
	Assert.Equals("", calls())
}

func (test *L3MembershipEnrollTest) TestAReservationWithoutAStoredPersonStillNeedsADisplayName() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	username := test.username()
	test.reservations().ForPerson(username, "kc-"+username)
	_, _, err := test.enroll().Enroll(AddMemberRequest{Username: username})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("displayName", invalid.Field)
}

func (test *L3MembershipEnrollTest) TestANameUsedByAnAliasIsADuplicate() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	username := test.username()
	test.reservations().ForAddress(username + "@alias-" + test.unique() + ".test")
	_, _, err := test.enroll().Enroll(AddMemberRequest{Username: username, DisplayName: "Ana", FirstPassword: "first-password"})
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
	Assert.Equals("", calls())
}

func (test *L3MembershipEnrollTest) TestANewPersonNeedsADisplayName() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	_, _, err := test.enroll().Enroll(AddMemberRequest{Username: test.username(), DisplayName: "  ", FirstPassword: "first-password"})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("displayName", invalid.Field)
	Assert.Equals("", calls())
}

func (test *L3MembershipEnrollTest) TestANewPersonNeedsAFirstPassword() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	_, _, err := test.enroll().Enroll(AddMemberRequest{Username: test.username(), DisplayName: "Ana", FirstPassword: " "})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("firstPassword", invalid.Field)
	Assert.Equals("", calls())
}

func (test *L3MembershipEnrollTest) TestAUsernameWithASpaceIsInvalid() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	_, _, err := test.enroll().Enroll(AddMemberRequest{Username: "ana example", DisplayName: "Ana", FirstPassword: "first-password"})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("username", invalid.Field)
}

func (test *L3MembershipEnrollTest) TestARoleNameIsNotAUsername() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	_, _, err := test.enroll().Enroll(AddMemberRequest{Username: "Postmaster", DisplayName: "Ana", FirstPassword: "first-password"})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("username", invalid.Field)
}

func (test *L3MembershipEnrollTest) TestAnAccountThatKeycloakAlreadyHasIsNeverTakenOver() {
	_, closeAll := test.keycloak(map[string]int{"POST /users": http.StatusConflict})
	defer closeAll()
	username := test.username()
	_, _, err := test.enroll().Enroll(AddMemberRequest{Username: username, DisplayName: "Ana", FirstPassword: "first-password"})
	reservation, _ := test.reservations().Current(username)
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
	Assert.Equals("", reservation.Owner)
}

func (test *L3MembershipEnrollTest) TestWhenKeycloakFailsNoNameIsReserved() {
	_, closeAll := test.keycloak(map[string]int{"POST /users": http.StatusInternalServerError})
	defer closeAll()
	username := test.username()
	_, _, err := test.enroll().Enroll(AddMemberRequest{Username: username, DisplayName: "Ana", FirstPassword: "first-password"})
	reservation, _ := test.reservations().Current(username)
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
	Assert.Equals("", reservation.Owner)
}

func (test *L3MembershipEnrollTest) keycloak(overrides map[string]int) (func() string, func()) {
	lock := &sync.Mutex{}
	lines := []string{}
	record := func(line string) {
		lock.Lock()
		defer lock.Unlock()
		lines = append(lines, line)
	}
	calls := func() string {
		lock.Lock()
		defer lock.Unlock()
		return strings.Join(lines, "\n")
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		test.answer(overrides, record, writer, request)
	}))
	settings := map[string]any{"yon": map[string]any{"directory": map[string]any{
		"keycloakurl": server.URL + "/idp", "admin": map[string]any{"client": "mesail-directory", "secret": "example-secret"},
	}}}
	encoded, _ := json.Marshal(settings)
	os.Setenv("YON_APPLICATION_JSON", string(encoded))
	return calls, server.Close
}

func (test *L3MembershipEnrollTest) answer(overrides map[string]int, record func(string), writer http.ResponseWriter, request *http.Request) {
	body, _ := io.ReadAll(request.Body)
	if strings.HasSuffix(request.URL.Path, "/token") {
		writer.Write([]byte(`{"access_token":"example-token"}`))
		return
	}
	key := request.Method + " " + strings.TrimPrefix(request.URL.Path, "/idp/admin/realms/mesail")
	record(key)
	status, overridden := overrides[key]
	if overridden {
		writer.WriteHeader(status)
		return
	}
	test.users(key, body, writer, request)
}

func (test *L3MembershipEnrollTest) users(key string, body []byte, writer http.ResponseWriter, request *http.Request) {
	user := struct {
		Username string `json:"username"`
	}{Username: request.URL.Query().Get("username")}
	json.Unmarshal(body, &user)
	if key == "GET /users" {
		encoded, _ := json.Marshal([]map[string]string{{"id": "kc-" + user.Username, "username": user.Username}})
		writer.Write(encoded)
		return
	}
	writer.Header().Set("Location", "http://"+request.Host+"/idp/admin/realms/mesail/users/kc-"+user.Username)
	writer.WriteHeader(http.StatusCreated)
}

func (test *L3MembershipEnrollTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *L3MembershipEnrollTest) username() string {
	return "u" + test.unique()
}

func (test *L3MembershipEnrollTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3MembershipEnrollTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipEnrollTest) reservations() *ReservationService {
	return &ReservationService{localParts: &LocalPartStorageService{documents: test.documents(), names: &CollectionNamesService{}}}
}

func (test *L3MembershipEnrollTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *L3MembershipEnrollTest) enroll() *PersonEnrollService {
	return &PersonEnrollService{people: test.people(), reservations: test.reservations(), accounts: test.accounts()}
}

func (test *L3MembershipEnrollTest) known(state string) string {
	username := test.username()
	personId := "kc-" + username
	Assert.Equals("<nil>", fmt.Sprint(test.reservations().ForPerson(username, personId)))
	_, err := test.people().Store(PersonDocument{PersonId: personId, Username: username, DisplayName: "Known Person", State: state})
	Assert.Equals("<nil>", fmt.Sprint(err))
	return username
}
