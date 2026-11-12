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
type L3PeopleRoutesTest struct {
	behaviors *BehaviorContext
}

func (test *L3PeopleRoutesTest) TestPutDefaultDomainChangesTheCallersDefault() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	first, second := test.hosted("active"), test.hosted("active")
	person := test.member(first, second)
	defer test.forget([]string{first, second}, person)
	err := test.meController().PutDefaultDomain(test.bearer(realm, person.PersonId, first, "mesail-member"), DefaultDomain{DomainId: second})
	stored, _, _ := test.people().ByPersonId(person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(second, stored.DefaultDomainId)
}

func (test *L3PeopleRoutesTest) TestPutDefaultDomainRefusesARequestWithoutAValidToken() {
	err := test.meController().PutDefaultDomain("Bearer forged", DefaultDomain{DomainId: "example.com"})
	Assert.IsTrue(test.unauthenticated(err), "want Unauthenticated")
}

func (test *L3PeopleRoutesTest) TestMemberChangesRefuseARequestWithoutAValidToken() {
	controller := &MembersController{admission: test.admission()}
	_, addErr := controller.PostMember("Bearer forged", AddMemberRequest{Username: "bo"})
	_, changeErr := controller.PatchMember("Bearer forged", "bo-id", MemberChange{Admin: true})
	removeErr := controller.DeleteMember("Bearer forged", "bo-id")
	Assert.IsTrue(test.unauthenticated(addErr), "add: want Unauthenticated")
	Assert.IsTrue(test.unauthenticated(changeErr), "change: want Unauthenticated")
	Assert.IsTrue(test.unauthenticated(removeErr), "remove: want Unauthenticated")
}

func (test *L3PeopleRoutesTest) TestMemberChangesRefuseAnAdminWhoIsNotAMemberOfTheDomain() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	first, second := test.hosted("active"), test.hosted("active")
	admin := test.admin(first)
	defer test.forget([]string{first, second}, admin)
	authorization := test.bearer(realm, admin.PersonId, second, "mesail-admin")
	controller := &MembersController{admission: test.admission()}
	_, addErr := controller.PostMember(authorization, AddMemberRequest{Username: "bo"})
	_, changeErr := controller.PatchMember(authorization, "bo-id", MemberChange{Admin: true})
	removeErr := controller.DeleteMember(authorization, "bo-id")
	Assert.IsTrue(test.forbidden(addErr), "add: want Forbidden")
	Assert.IsTrue(test.forbidden(changeErr), "change: want Forbidden")
	Assert.IsTrue(test.forbidden(removeErr), "remove: want Forbidden")
}

func (test *L3PeopleRoutesTest) TestAddingAMemberToANamedDomainRefusesARequestWithoutAValidToken() {
	_, err := (&DomainsController{callers: &CallerService{}}).PostMember("Bearer forged", "example.com", AddMemberRequest{Username: "bo"})
	Assert.IsTrue(test.unauthenticated(err), "want Unauthenticated")
}

func (test *L3PeopleRoutesTest) TestAddingAMemberToANamedDomainRefusesADomainAdmin() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	authorization := test.bearer(realm, "admin-"+test.unique(), "example.com", "mesail-admin")
	_, err := (&DomainsController{callers: &CallerService{}}).PostMember(authorization, "example.com", AddMemberRequest{Username: "bo"})
	Assert.IsTrue(test.forbidden(err), "want Forbidden")
}

func (test *L3PeopleRoutesTest) TestAPasswordPolicyFailureNamesThePassword() {
	answer := (&DirectoryErrorHandler{}).PasswordPolicy(&PasswordPolicy{Message: "the password is too short"})
	Assert.Equals("password_policy", answer.Code)
	Assert.Equals("password", answer.Field)
	Assert.Equals("the password is too short", answer.Message)
}

func (test *L3PeopleRoutesTest) TestRemovingTheLastAdminIsALastAdminConflict() {
	answer := (&DirectoryErrorHandler{}).LastAdmin(&LastAdmin{Message: "a domain must keep at least one admin"})
	Assert.Equals("last_admin", answer.Code)
	Assert.Equals("a domain must keep at least one admin", answer.Message)
}

func (test *L3PeopleRoutesTest) meController() *MeController {
	return &MeController{callers: &CallerService{}, people: test.personService()}
}

func (test *L3PeopleRoutesTest) unauthenticated(err error) bool {
	var refused *Unauthenticated
	return errors.As(err, &refused)
}

func (test *L3PeopleRoutesTest) forbidden(err error) bool {
	var refused *Forbidden
	return errors.As(err, &refused)
}

func (test *L3PeopleRoutesTest) personService() *PersonService {
	return &PersonService{people: test.people(), access: test.access(), accounts: test.accounts(), writer: test.writer(), domains: test.domains()}
}

func (test *L3PeopleRoutesTest) admission() *AdmissionService {
	tenants := &TenantService{domains: test.domains()}
	return &AdmissionService{callers: &CallerService{}, people: test.people(), tenants: tenants}
}

func (test *L3PeopleRoutesTest) access() *PersonAccessService {
	return &PersonAccessService{people: test.people(), accounts: test.accounts()}
}

func (test *L3PeopleRoutesTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *L3PeopleRoutesTest) writer() *MemberWriterService {
	return &MemberWriterService{members: test.memberStorage(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: test.relay()}
}

func (test *L3PeopleRoutesTest) relay() *OutboxRelayService {
	store := &OutboxStoreService{domains: test.domains(), members: test.memberStorage(), names: &CollectionNamesService{}}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
}

func (test *L3PeopleRoutesTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleRoutesTest) memberStorage() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleRoutesTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleRoutesTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleRoutesTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3PeopleRoutesTest) hosted(state string) string {
	domainId := "l3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: state})))
	return domainId
}

func (test *L3PeopleRoutesTest) member(domainIds ...string) PersonDocument {
	return test.joined(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Person " + test.unique(), State: "active"}, false, domainIds)
}

func (test *L3PeopleRoutesTest) admin(domainIds ...string) PersonDocument {
	return test.joined(PersonDocument{PersonId: "a-" + test.unique(), Username: "a" + test.unique(), DisplayName: "Admin " + test.unique(), State: "active"}, true, domainIds)
}

func (test *L3PeopleRoutesTest) joined(person PersonDocument, admin bool, domainIds []string) PersonDocument {
	for _, domainId := range domainIds {
		person = PersonRules{}.WithAdmin(PersonRules{}.With(person, domainId), domainId, admin)
		_, err := test.writer().Join(person, domainId, admin)
		Assert.Equals("<nil>", fmt.Sprint(err))
	}
	return test.stored(person)
}

func (test *L3PeopleRoutesTest) stored(person PersonDocument) PersonDocument {
	saved, err := test.people().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return saved
}

func (test *L3PeopleRoutesTest) forget(domainIds []string, people ...PersonDocument) {
	for _, person := range people {
		test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
	}
	for _, domainId := range domainIds {
		domain, _, _ := test.domains().Find(domainId)
		test.domains().Delete(domain)
		(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
	}
}

func (test *L3PeopleRoutesTest) realm() oidc.SigningRealm {
	return DirectoryTestSetup{}.SignedRealm()
}

func (test *L3PeopleRoutesTest) bearer(realm oidc.SigningRealm, subject string, domainId string, realmRoles ...string) string {
	return DirectoryTestSetup{}.PersonBearer(realm, subject, domainId, realmRoles...)
}

func (test *L3PeopleRoutesTest) configure(realm oidc.SigningRealm, base string) {
	os.Setenv("YON_APPLICATION_JSON", DirectoryTestSetup{}.ApplicationJson(realm, base))
}

func (test *L3PeopleRoutesTest) keycloak(operator bool, passwordStatus int) (string, func() string, func()) {
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

func (test *L3PeopleRoutesTest) answer(operator bool, passwordStatus int, record func(string), writer http.ResponseWriter, request *http.Request) {
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

func (test *L3PeopleRoutesTest) adminAnswer(operator bool, passwordStatus int, method string, path string) (int, string) {
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

func (test *L3PeopleRoutesTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
