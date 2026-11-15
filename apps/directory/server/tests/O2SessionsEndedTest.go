// tachyon: example secrets
import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"time"

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type O2SessionsEndedTest struct {
	behaviors *BehaviorContext
}

func (test *O2SessionsEndedTest) endedAt(personId string) string {
	stored, _, err := test.people().ByPersonId(personId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return stored.SessionsEndedAt
}

func (test *O2SessionsEndedTest) endedSince(personId string, start time.Time) bool {
	ended, err := time.Parse(time.RFC3339Nano, test.endedAt(personId))
	return err == nil && !ended.Before(start)
}

func (test *O2SessionsEndedTest) TestAPasswordResetEndsTheSessions() {
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	start := time.Now()
	err := test.personService().ResetPassword(test.operator(), person.PersonId, PasswordReset{NewPassword: "a-long-new-password"})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(test.endedSince(person.PersonId, start), "want sessionsEndedAt set")
}

func (test *O2SessionsEndedTest) TestARefusedPasswordResetKeepsTheSessions() {
	base, _, closeServer := test.keycloak(false, http.StatusBadRequest)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.personService().ResetPassword(test.operator(), person.PersonId, PasswordReset{NewPassword: "short"})
	Assert.IsTrue(err != nil, "want the policy failure")
	Assert.Equals("", test.endedAt(person.PersonId))
}

func (test *O2SessionsEndedTest) TestRemovingTheTwoStepCodeEndsTheSessions() {
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	start := time.Now()
	err := test.personService().RemoveTotp(test.operator(), person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(test.endedSince(person.PersonId, start), "want sessionsEndedAt set")
}

func (test *O2SessionsEndedTest) TestRemovingAllPasskeysEndsTheSessions() {
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	start := time.Now()
	err := test.personService().RemovePasskeys(test.operator(), person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(test.endedSince(person.PersonId, start), "want sessionsEndedAt set")
}

func (test *O2SessionsEndedTest) TestDisablingAPersonEndsTheSessions() {
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	start := time.Now()
	_, err := test.personService().Change(test.operator(), person.PersonId, PersonChange{State: "disabled"})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(test.endedSince(person.PersonId, start), "want sessionsEndedAt set")
}

func (test *O2SessionsEndedTest) TestRenamingAPersonKeepsTheSessions() {
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	_, err := test.personService().Change(test.operator(), person.PersonId, PersonChange{DisplayName: "New Name"})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("", test.endedAt(person.PersonId))
}

func (test *O2SessionsEndedTest) TestDeletingAPersonEndsTheSessions() {
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	start := time.Now()
	err := test.personService().Delete(person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(test.endedSince(person.PersonId, start), "want sessionsEndedAt set")
}

func (test *O2SessionsEndedTest) TestEndSessionsSetsTheTimeAndTheRecordShowsIt() {
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	start := time.Now()
	err := test.personService().EndSessions(person.PersonId)
	record, recordErr := test.personService().Record(person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint(recordErr))
	Assert.Equals(test.endedAt(person.PersonId), record.SessionsEndedAt)
	Assert.IsTrue(test.endedSince(person.PersonId, start), "want sessionsEndedAt set")
}

func (test *O2SessionsEndedTest) TestEndSessionsOfAnUnknownPersonIsNotFound() {
	err := test.personService().EndSessions("nobody-" + test.unique())
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *O2SessionsEndedTest) controller() *PeopleController {
	return &PeopleController{people: &PersonRouteService{callers: &CallerService{}, people: test.personService()}}
}

func (test *O2SessionsEndedTest) TestTheRouteAcceptsAnInstallWideServiceToken() {
	realm := DirectoryTestSetup{}.SignedRealm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	os.Setenv("YON_APPLICATION_JSON", DirectoryTestSetup{}.ApplicationJson(realm, base))
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.controller().PostSessionsEnded(DirectoryTestSetup{}.ServiceBearer(realm, "*"), person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(test.endedAt(person.PersonId) != "", "want sessionsEndedAt set")
}

func (test *O2SessionsEndedTest) TestTheRouteRefusesAServiceTokenForOneDomain() {
	realm := DirectoryTestSetup{}.SignedRealm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	os.Setenv("YON_APPLICATION_JSON", DirectoryTestSetup{}.ApplicationJson(realm, base))
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.controller().PostSessionsEnded(DirectoryTestSetup{}.ServiceBearer(realm, domainId), person.PersonId)
	Assert.IsTrue(err != nil, "want a refusal")
	Assert.Equals("", test.endedAt(person.PersonId))
}

func (test *O2SessionsEndedTest) TestTheRouteRefusesAForgedToken() {
	err := test.controller().PostSessionsEnded("Bearer forged", "p-1")
	var refused *Unauthenticated
	Assert.IsTrue(errors.As(err, &refused), "want Unauthenticated")
}

func (test *O2SessionsEndedTest) personService() *PersonService {
	return &PersonService{people: test.people(), access: test.access(), accounts: test.accounts(), writer: test.writer(), domains: test.domains()}
}

func (test *O2SessionsEndedTest) access() *PersonAccessService {
	return &PersonAccessService{people: test.people(), accounts: test.accounts()}
}

func (test *O2SessionsEndedTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *O2SessionsEndedTest) writer() *MemberWriterService {
	return &MemberWriterService{members: test.memberStorage(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: test.relay()}
}

func (test *O2SessionsEndedTest) relay() *OutboxRelayService {
	store := &OutboxStoreService{domains: test.domains(), members: test.memberStorage(), names: &CollectionNamesService{}}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
}

func (test *O2SessionsEndedTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *O2SessionsEndedTest) memberStorage() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *O2SessionsEndedTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *O2SessionsEndedTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *O2SessionsEndedTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *O2SessionsEndedTest) hosted(state string) string {
	domainId := "l3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: state})))
	return domainId
}

func (test *O2SessionsEndedTest) member(domainIds ...string) PersonDocument {
	return test.joined(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Person " + test.unique(), State: "active"}, false, domainIds)
}

func (test *O2SessionsEndedTest) joined(person PersonDocument, admin bool, domainIds []string) PersonDocument {
	for _, domainId := range domainIds {
		person = PersonRules{}.WithAdmin(PersonRules{}.With(person, domainId), domainId, admin)
		_, err := test.writer().Join(person, domainId, admin)
		Assert.Equals("<nil>", fmt.Sprint(err))
	}
	return test.stored(person)
}

func (test *O2SessionsEndedTest) stored(person PersonDocument) PersonDocument {
	saved, err := test.people().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return saved
}

func (test *O2SessionsEndedTest) forget(domainIds []string, people ...PersonDocument) {
	for _, person := range people {
		test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
	}
	for _, domainId := range domainIds {
		domain, _, _ := test.domains().Find(domainId)
		test.domains().Delete(domain)
		(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId, &WriteBudget{Left: 2, Deadline: time.Now().Add(time.Minute)})
	}
}

func (test *O2SessionsEndedTest) operator() oidc.Caller {
	return oidc.Caller{DomainId: "*", MemberId: "operator-" + test.unique(), Roles: []string{"Operator"}}
}

func (test *O2SessionsEndedTest) configure(base string) {
	settings := `{"yon":{"directory":{"keycloakurl":%q,"admin":{"client":"mesail-directory","secret":"example-secret"}}}}`
	os.Setenv("YON_APPLICATION_JSON", fmt.Sprintf(settings, base+"/idp"))
}

func (test *O2SessionsEndedTest) keycloak(operator bool, passwordStatus int) (string, func() string, func()) {
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
		test.answer(operator, passwordStatus, record, writer, request)
	}))
	return server.URL, calls, server.Close
}

func (test *O2SessionsEndedTest) answer(operator bool, passwordStatus int, record func(string), writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	if strings.HasSuffix(path, "/protocol/openid-connect/token") {
		writer.Write([]byte(`{"access_token":"example-token"}`))
		return
	}
	record(request.Method + " " + path)
	status, body := test.adminAnswer(operator, passwordStatus, request.Method, path)
	writer.WriteHeader(status)
	writer.Write([]byte(body))
}

func (test *O2SessionsEndedTest) adminAnswer(operator bool, passwordStatus int, method string, path string) (int, string) {
	roles := map[bool]string{true: `[{"name":"mesail-operator"}]`, false: `[{"name":"mesail-member"}]`}
	switch {
	case strings.HasSuffix(path, "/reset-password"):
		return passwordStatus, `{"errorMessage":"the password is too short"}`
	case strings.HasSuffix(path, "/role-mappings/realm/composite"):
		return http.StatusOK, roles[operator]
	case strings.HasSuffix(path, "/credentials"):
		return http.StatusOK, `[{"id":"otp-1","type":"otp"},{"id":"password-1","type":"password"}]`
	case method == http.MethodGet && !strings.HasSuffix(path, "/logout"):
		return http.StatusOK, `{"id":"user","enabled":true,"attributes":{}}`
	}
	return http.StatusNoContent, ""
}

func (test *O2SessionsEndedTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
