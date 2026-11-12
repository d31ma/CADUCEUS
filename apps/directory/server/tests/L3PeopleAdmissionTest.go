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

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type L3PeopleAdmissionTest struct {
	behaviors *BehaviorContext
}

func (test *L3PeopleAdmissionTest) TestAnActiveMemberOfAHostedDomainIsAdmitted() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	admission := test.admission().Admit(test.bearer(realm, person.PersonId, domainId, "mesail-member"))
	Assert.Equals("<nil>", fmt.Sprint(admission.Refusal))
	Assert.Equals(person.PersonId, admission.Caller.MemberId)
	Assert.Equals(domainId, admission.Caller.DomainId)
}

func (test *L3PeopleAdmissionTest) TestAPersonWhoIsNotAMemberOfTheTokenDomainIsForbidden() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	first, second := test.hosted("active"), test.hosted("active")
	person := test.member(first)
	defer test.forget([]string{first, second}, person)
	admission := test.admission().Admit(test.bearer(realm, person.PersonId, second, "mesail-member"))
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(admission.Refusal, &forbidden), "want Forbidden")
}

func (test *L3PeopleAdmissionTest) TestADisabledPersonIsForbidden() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	domainId := test.hosted("active")
	person := test.stored(PersonRules{}.With(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), State: "disabled"}, domainId))
	defer test.forget([]string{domainId}, person)
	admission := test.admission().Admit(test.bearer(realm, person.PersonId, domainId, "mesail-member"))
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(admission.Refusal, &forbidden), "want Forbidden")
}

func (test *L3PeopleAdmissionTest) TestAMemberOfADisabledDomainIsForbidden() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	domainId := test.hosted("disabled")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	admission := test.admission().Admit(test.bearer(realm, person.PersonId, domainId, "mesail-member"))
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(admission.Refusal, &forbidden), "want Forbidden")
}

func (test *L3PeopleAdmissionTest) TestACallerWithoutAPersonIsForbiddenAndNothingIsStored() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	domainId := test.hosted("active")
	defer test.forget([]string{domainId})
	subject := "token-" + test.unique()
	admission := test.admission().Admit(test.bearer(realm, subject, domainId, "mesail-member"))
	_, found, _ := test.memberStorage().Find(domainId, subject)
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(admission.Refusal, &forbidden), "want Forbidden")
	Assert.IsTrue(!found, "a caller without a person was stored as a member")
}

func (test *L3PeopleAdmissionTest) TestATokenThatCannotBeVerifiedIsUnauthenticated() {
	admission := test.admission().Admit("Bearer forged")
	var refused *Unauthenticated
	Assert.IsTrue(errors.As(admission.Refusal, &refused), "want Unauthenticated")
}

func (test *L3PeopleAdmissionTest) admission() *AdmissionService {
	tenants := &TenantService{domains: test.domains()}
	return &AdmissionService{callers: &CallerService{}, people: test.people(), tenants: tenants}
}

func (test *L3PeopleAdmissionTest) writer() *MemberWriterService {
	return &MemberWriterService{members: test.memberStorage(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: test.relay()}
}

func (test *L3PeopleAdmissionTest) relay() *OutboxRelayService {
	store := &OutboxStoreService{domains: test.domains(), members: test.memberStorage(), names: &CollectionNamesService{}}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
}

func (test *L3PeopleAdmissionTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleAdmissionTest) memberStorage() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleAdmissionTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleAdmissionTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleAdmissionTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3PeopleAdmissionTest) hosted(state string) string {
	domainId := "l3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: state})))
	return domainId
}

func (test *L3PeopleAdmissionTest) member(domainIds ...string) PersonDocument {
	return test.joined(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Person " + test.unique(), State: "active"}, false, domainIds)
}

func (test *L3PeopleAdmissionTest) joined(person PersonDocument, admin bool, domainIds []string) PersonDocument {
	for _, domainId := range domainIds {
		person = PersonRules{}.WithAdmin(PersonRules{}.With(person, domainId), domainId, admin)
		_, err := test.writer().Join(person, domainId, admin)
		Assert.Equals("<nil>", fmt.Sprint(err))
	}
	return test.stored(person)
}

func (test *L3PeopleAdmissionTest) stored(person PersonDocument) PersonDocument {
	saved, err := test.people().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return saved
}

func (test *L3PeopleAdmissionTest) forget(domainIds []string, people ...PersonDocument) {
	for _, person := range people {
		test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
	}
	for _, domainId := range domainIds {
		domain, _, _ := test.domains().Find(domainId)
		test.domains().Delete(domain)
		(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
	}
}

func (test *L3PeopleAdmissionTest) realm() oidc.SigningRealm {
	return DirectoryTestSetup{}.SignedRealm()
}

func (test *L3PeopleAdmissionTest) bearer(realm oidc.SigningRealm, subject string, domainId string, realmRoles ...string) string {
	return DirectoryTestSetup{}.PersonBearer(realm, subject, domainId, realmRoles...)
}

func (test *L3PeopleAdmissionTest) configure(realm oidc.SigningRealm, base string) {
	os.Setenv("YON_APPLICATION_JSON", DirectoryTestSetup{}.ApplicationJson(realm, base))
}

func (test *L3PeopleAdmissionTest) keycloak(operator bool, passwordStatus int) (string, func() string, func()) {
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

func (test *L3PeopleAdmissionTest) answer(operator bool, passwordStatus int, record func(string), writer http.ResponseWriter, request *http.Request) {
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

func (test *L3PeopleAdmissionTest) adminAnswer(operator bool, passwordStatus int, method string, path string) (int, string) {
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

func (test *L3PeopleAdmissionTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
