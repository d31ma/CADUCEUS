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

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type D2RecoveryTest struct {
	behaviors *BehaviorContext
}

func (test *D2RecoveryTest) TestAnAddThatFailedAfterKeycloakMadeTheUserFinishesOnTheNextAdd() {
	calls, tag, closeServer := test.keycloak(map[string]bool{"failCreate": true}, map[string][]string{})
	defer closeServer()
	domain := test.hosted()
	defer test.forget(domain)
	username := "u" + test.unique()
	_, firstErr := test.add(domain, username)
	pending := test.localPart(username)
	madeTag := tag(username)
	added, err := test.add(domain, username)
	person, _, _ := test.people().ByPersonId("kc-" + username)
	defer test.forgetPerson(person)
	reservation := test.localPart(username)
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(firstErr, &unavailable), "want the first add to fail")
	Assert.Equals("", pending.Owner)
	Assert.Equals(32, len(pending.AddRequest))
	Assert.Equals(pending.AddRequest, madeTag)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("kc-"+username, added.Member.MemberId)
	Assert.IsTrue(!added.ExistingPerson, "want a new person")
	Assert.Equals(domain, strings.Join(person.DomainIds, ","))
	Assert.Equals("person", reservation.Owner)
	Assert.Equals("kc-"+username, reservation.PersonId)
	Assert.IsTrue(strings.Contains(calls(), "PUT /users/kc-"+username+"/reset-password"), "want the first password of this add set")
}

func (test *D2RecoveryTest) TestAKeycloakUserWithoutTheTagIsNeverTakenOver() {
	username := "u" + test.unique()
	calls, _, closeServer := test.keycloak(map[string]bool{}, map[string][]string{username: {}})
	defer closeServer()
	domain := test.hosted()
	defer test.forget(domain)
	_, err := test.add(domain, username)
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
	Assert.Equals("", test.localPart(username).Owner)
	Assert.IsTrue(!strings.Contains(calls(), "reset-password"), "the password of a foreign user must not change")
}

func (test *D2RecoveryTest) TestAKeycloakUserFromAnotherAddIsNeverTakenOver() {
	username := "u" + test.unique()
	_, _, closeServer := test.keycloak(map[string]bool{}, map[string][]string{username: {"another-request"}})
	defer closeServer()
	domain := test.hosted()
	defer test.forget(domain)
	_, err := test.add(domain, username)
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *D2RecoveryTest) TestAUserThatVanishesBeforeTheRecoveryIsADuplicate() {
	username := "u" + test.unique()
	_, _, closeServer := test.keycloak(map[string]bool{"failLookups": true}, map[string][]string{username: {"0f3a"}})
	defer closeServer()
	_, err := test.accounts().Create(username, "first-password", "0f3a")
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *D2RecoveryTest) TestARecoveryReadThatFailsIsUnavailable() {
	username := "u" + test.unique()
	_, _, closeServer := test.keycloak(map[string]bool{"failReads": true}, map[string][]string{username: {"0f3a"}})
	defer closeServer()
	_, err := test.accounts().Create(username, "first-password", "0f3a")
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
}

func (test *D2RecoveryTest) TestAPendingAddKeepsItsRequestAcrossRetries() {
	username := "u" + test.unique()
	first, err := test.reservations().Pending(username)
	second, secondErr := test.reservations().Pending(username)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint(secondErr))
	Assert.Equals(first, second)
	Assert.Equals(first, test.localPart(username).AddRequest)
}

func (test *D2RecoveryTest) TestAnAliasCannotTakeAPrefixThatAPersonAddIsUsing() {
	domain := test.hosted()
	defer test.forget(domain)
	username := "u" + test.unique()
	test.reservations().Pending(username)
	err := test.reservations().ForAddress(username + "@" + domain)
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *D2RecoveryTest) add(domain string, username string) (AddedMember, error) {
	request := AddMemberRequest{Username: username, DisplayName: "Person " + username, FirstPassword: "first-password"}
	return test.membership().Add(oidc.Caller{DomainId: domain, MemberId: "kc-op-" + test.unique(), Roles: []string{"Operator"}}, domain, request)
}

func (test *D2RecoveryTest) keycloak(flags map[string]bool, tags map[string][]string) (func() string, func(string) string, func()) {
	lock := &sync.Mutex{}
	lines := []string{}
	users := map[string][]string{}
	for username, tag := range tags {
		users[username] = tag
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/token") {
			writer.Write([]byte(`{"access_token":"example-token"}`))
			return
		}
		lock.Lock()
		defer lock.Unlock()
		path := strings.TrimPrefix(request.URL.Path, "/idp/admin/realms/mesail")
		lines = append(lines, request.Method+" "+path)
		body, _ := io.ReadAll(request.Body)
		test.answer(flags, users, path, body, writer, request)
	}))
	settings := `{"yon":{"directory":{"keycloakurl":%q,"admin":{"client":"mesail-directory","secret":"example-secret"}}}}`
	os.Setenv("YON_APPLICATION_JSON", fmt.Sprintf(settings, server.URL+"/idp"))
	calls := func() string {
		lock.Lock()
		defer lock.Unlock()
		return strings.Join(lines, "\n")
	}
	tag := func(username string) string {
		lock.Lock()
		defer lock.Unlock()
		return strings.Join(users[username], ",")
	}
	return calls, tag, server.Close
}

func (test *D2RecoveryTest) answer(flags map[string]bool, users map[string][]string, path string, body []byte, writer http.ResponseWriter, request *http.Request) {
	switch {
	case request.Method == http.MethodPost && path == "/users":
		test.create(flags, users, body, writer, request)
	case request.Method == http.MethodGet && path == "/users":
		test.search(flags, users, request.URL.Query().Get("username"), writer)
	case request.Method == http.MethodGet:
		test.read(flags, users, strings.TrimPrefix(path, "/users/kc-"), writer)
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}

func (test *D2RecoveryTest) create(flags map[string]bool, users map[string][]string, body []byte, writer http.ResponseWriter, request *http.Request) {
	user := struct {
		Username   string              `json:"username"`
		Attributes map[string][]string `json:"attributes"`
	}{}
	json.Unmarshal(body, &user)
	if _, taken := users[user.Username]; taken {
		writer.WriteHeader(http.StatusConflict)
		return
	}
	users[user.Username] = user.Attributes["mesail.addRequest"]
	if flags["failCreate"] {
		flags["failCreate"] = false
		writer.WriteHeader(http.StatusBadGateway)
		return
	}
	writer.Header().Set("Location", "http://"+request.Host+"/idp/admin/realms/mesail/users/kc-"+user.Username)
	writer.WriteHeader(http.StatusCreated)
}

func (test *D2RecoveryTest) search(flags map[string]bool, users map[string][]string, username string, writer http.ResponseWriter) {
	if _, found := users[username]; !found || flags["failLookups"] {
		writer.Write([]byte(`[]`))
		return
	}
	encoded, _ := json.Marshal([]map[string]string{{"id": "kc-" + username, "username": username}})
	writer.Write(encoded)
}

func (test *D2RecoveryTest) read(flags map[string]bool, users map[string][]string, username string, writer http.ResponseWriter) {
	if flags["failReads"] {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	encoded, _ := json.Marshal(map[string]any{"id": "kc-" + username, "enabled": true, "attributes": map[string][]string{"mesail.addRequest": users[username]}})
	writer.Write(encoded)
}

func (test *D2RecoveryTest) localPart(localPart string) LocalPartDocument {
	document, _ := test.localParts().Find(localPart)
	return document
}

func (test *D2RecoveryTest) hosted() string {
	domainId := "d2-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	return domainId
}

func (test *D2RecoveryTest) forget(domainId string) {
	domain, _, _ := test.domains().Find(domainId)
	test.domains().Delete(domain)
	(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
}

func (test *D2RecoveryTest) forgetPerson(person PersonDocument) {
	test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
}

func (test *D2RecoveryTest) membership() *MembershipService {
	access := &PersonAccessService{people: test.people(), accounts: test.accounts()}
	enroll := &PersonEnrollService{people: test.people(), reservations: test.reservations(), accounts: test.accounts()}
	return &MembershipService{tenants: &TenantService{domains: test.domains()}, enroll: enroll, people: test.people(), writer: test.writer(), access: access}
}

func (test *D2RecoveryTest) writer() *MemberWriterService {
	members := &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
	store := &OutboxStoreService{domains: test.domains(), members: members, names: &CollectionNamesService{}}
	relay := &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
	addresses := &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
	return &MemberWriterService{members: members, addresses: addresses, events: &EventService{settings: &SettingsService{}}, relay: relay}
}

func (test *D2RecoveryTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *D2RecoveryTest) reservations() *ReservationService {
	return &ReservationService{localParts: test.localParts()}
}

func (test *D2RecoveryTest) localParts() *LocalPartStorageService {
	return &LocalPartStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2RecoveryTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2RecoveryTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2RecoveryTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *D2RecoveryTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
