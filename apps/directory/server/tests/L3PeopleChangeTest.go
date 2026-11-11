// tachyon: example secrets
import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
type L3PeopleChangeTest struct {
	behaviors *BehaviorContext
}

func (test *L3PeopleChangeTest) TestAnOperatorRenamesAPersonInEveryDomain() {
	base, calls, closeServer := test.keycloak(test.key(), false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	first, second := test.hosted("active"), test.hosted("active")
	person := test.member(first, second)
	defer test.forget([]string{first, second}, person)
	summary, err := test.personService().Change(test.operator(), person.PersonId, PersonChange{DisplayName: "  New Name  "})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("New Name", summary.DisplayName)
	Assert.Equals("active", summary.State)
	Assert.Equals("New Name", test.memberDocument(first, person.PersonId).DisplayName)
	Assert.Equals("New Name", test.memberDocument(second, person.PersonId).DisplayName)
	Assert.Equals("", calls())
}

func (test *L3PeopleChangeTest) TestDisablingAPersonDisablesTheAccountAndEveryMember() {
	base, calls, closeServer := test.keycloak(test.key(), false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	summary, err := test.personService().Change(test.operator(), person.PersonId, PersonChange{State: "disabled"})
	stored, _, _ := test.people().ByPersonId(person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("disabled", summary.State)
	Assert.Equals("disabled", stored.State)
	Assert.Equals("disabled", test.memberDocument(domainId, person.PersonId).State)
	Assert.IsTrue(strings.Contains(calls(), "PUT /idp/admin/realms/mesail/users/"+person.PersonId), "want the account to be updated")
}

func (test *L3PeopleChangeTest) TestAnUnknownStateIsRefused() {
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	_, err := test.personService().Change(test.operator(), person.PersonId, PersonChange{State: "deleted"})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("state", invalid.Field)
}

func (test *L3PeopleChangeTest) TestChangingAStateWithoutAnIdentityServerIsUnavailable() {
	test.configureWithoutKeycloak()
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	_, err := test.personService().Change(test.operator(), person.PersonId, PersonChange{State: "disabled"})
	stored, _, _ := test.people().ByPersonId(person.PersonId)
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
	Assert.Equals("active", stored.State)
}

func (test *L3PeopleChangeTest) TestAnAdminOfEveryDomainChangesAPersonWhoIsNotAnOperator() {
	base, calls, closeServer := test.keycloak(test.key(), false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	first, second := test.hosted("active"), test.hosted("active")
	admin := test.admin(first, second)
	person := test.member(first, second)
	defer test.forget([]string{first, second}, admin, person)
	summary, err := test.personService().Change(test.domainAdmin(admin, first), person.PersonId, PersonChange{DisplayName: "Changed By Admin"})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("Changed By Admin", summary.DisplayName)
	Assert.IsTrue(strings.Contains(calls(), "GET /idp/admin/realms/mesail/users/"+person.PersonId+"/role-mappings/realm/composite"), "want a role check")
}

func (test *L3PeopleChangeTest) TestAnAdminMayNotChangeAnOperator() {
	base, _, closeServer := test.keycloak(test.key(), true, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	admin := test.admin(domainId)
	person := test.member(domainId)
	defer test.forget([]string{domainId}, admin, person)
	_, err := test.personService().Change(test.domainAdmin(admin, domainId), person.PersonId, PersonChange{DisplayName: "Taken Over"})
	stored, _, _ := test.people().ByPersonId(person.PersonId)
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
	Assert.Equals(person.DisplayName, stored.DisplayName)
}

func (test *L3PeopleChangeTest) TestAnAdminOfOnlySomeDomainsOfThePersonIsForbidden() {
	base, calls, closeServer := test.keycloak(test.key(), false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	first, second := test.hosted("active"), test.hosted("active")
	admin := test.admin(first)
	person := test.member(first, second)
	defer test.forget([]string{first, second}, admin, person)
	_, err := test.personService().Change(test.domainAdmin(admin, first), person.PersonId, PersonChange{DisplayName: "Taken Over"})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
	Assert.Equals("", calls())
}

func (test *L3PeopleChangeTest) TestTheRoleCheckFailingIsUnavailable() {
	test.configureWithoutKeycloak()
	domainId := test.hosted("active")
	admin := test.admin(domainId)
	person := test.member(domainId)
	defer test.forget([]string{domainId}, admin, person)
	_, err := test.personService().Change(test.domainAdmin(admin, domainId), person.PersonId, PersonChange{DisplayName: "Taken Over"})
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
}

func (test *L3PeopleChangeTest) TestAChangeOfADeletedPersonIsNotFound() {
	person := test.stored(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), State: "deleted"})
	defer test.forget([]string{}, person)
	_, err := test.personService().Change(test.operator(), person.PersonId, PersonChange{DisplayName: "Back"})
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3PeopleChangeTest) personService() *PersonService {
	return &PersonService{people: test.people(), access: test.access(), accounts: test.accounts(), writer: test.writer()}
}

func (test *L3PeopleChangeTest) access() *PersonAccessService {
	return &PersonAccessService{people: test.people(), accounts: test.accounts()}
}

func (test *L3PeopleChangeTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *L3PeopleChangeTest) writer() *MemberWriterService {
	return &MemberWriterService{members: test.memberStorage(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: test.relay()}
}

func (test *L3PeopleChangeTest) relay() *OutboxRelayService {
	store := &OutboxStoreService{domains: test.domains(), members: test.memberStorage(), names: &CollectionNamesService{}}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
}

func (test *L3PeopleChangeTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleChangeTest) memberStorage() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleChangeTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleChangeTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleChangeTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3PeopleChangeTest) hosted(state string) string {
	domainId := "l3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: state})))
	return domainId
}

func (test *L3PeopleChangeTest) member(domainIds ...string) PersonDocument {
	return test.joined(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Person " + test.unique(), State: "active"}, false, domainIds)
}

func (test *L3PeopleChangeTest) admin(domainIds ...string) PersonDocument {
	return test.joined(PersonDocument{PersonId: "a-" + test.unique(), Username: "a" + test.unique(), DisplayName: "Admin " + test.unique(), State: "active"}, true, domainIds)
}

func (test *L3PeopleChangeTest) joined(person PersonDocument, admin bool, domainIds []string) PersonDocument {
	for _, domainId := range domainIds {
		person = PersonRules{}.WithAdmin(PersonRules{}.With(person, domainId), domainId, admin)
		_, err := test.writer().Join(person, domainId, admin)
		Assert.Equals("<nil>", fmt.Sprint(err))
	}
	return test.stored(person)
}

func (test *L3PeopleChangeTest) stored(person PersonDocument) PersonDocument {
	saved, err := test.people().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return saved
}

func (test *L3PeopleChangeTest) forget(domainIds []string, people ...PersonDocument) {
	for _, person := range people {
		test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
	}
	for _, domainId := range domainIds {
		domain, _, _ := test.domains().Find(domainId)
		test.domains().Delete(domain)
		(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
	}
}

func (test *L3PeopleChangeTest) memberDocument(domainId string, personId string) MemberDocument {
	member, _, _ := test.memberStorage().Find(domainId, personId)
	return member
}

func (test *L3PeopleChangeTest) operator() oidc.Caller {
	return oidc.Caller{DomainId: "*", MemberId: "operator-" + test.unique(), Roles: []string{"Operator"}}
}

func (test *L3PeopleChangeTest) domainAdmin(admin PersonDocument, domainId string) oidc.Caller {
	return oidc.Caller{DomainId: domainId, MemberId: admin.PersonId, Username: admin.Username, Roles: []string{"DomainAdmin"}}
}

func (test *L3PeopleChangeTest) key() oidc.SigningKey {
	key, err := oidc.SigningKey{}.Generated()
	Assert.Equals("<nil>", fmt.Sprint(err))
	return key
}

func (test *L3PeopleChangeTest) configure(base string) {
	settings := `{"yon":{"oidc":{"issuer":%q,"jwks":%q,"audience":%q},"directory":{"keycloakurl":%q,"admin":{"client":"mesail-directory","secret":"example-secret"}}}}`
	os.Setenv("YON_APPLICATION_JSON", fmt.Sprintf(settings, test.issuer(base), base+"/jwks/"+test.unique(), test.audience(), base+"/idp"))
}

func (test *L3PeopleChangeTest) configureWithoutKeycloak() {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{}}`)
}

func (test *L3PeopleChangeTest) issuer(base string) string {
	return base + "/realms/mesail-test"
}

func (test *L3PeopleChangeTest) audience() string {
	return "mesail-directory-test"
}

func (test *L3PeopleChangeTest) keycloak(key oidc.SigningKey, operator bool, passwordStatus int) (string, func() string, func()) {
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
		test.answer(key, operator, passwordStatus, record, writer, request)
	}))
	return server.URL, calls, server.Close
}

func (test *L3PeopleChangeTest) answer(key oidc.SigningKey, operator bool, passwordStatus int, record func(string), writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	if strings.HasPrefix(path, "/jwks/") {
		keys, _ := oidc.TokenSigner{Current: key}.KeySet()
		writer.Write(test.keySet(keys))
		return
	}
	if strings.HasSuffix(path, "/protocol/openid-connect/token") {
		writer.Write([]byte(`{"access_token":"example-token"}`))
		return
	}
	record(request.Method + " " + path)
	status, body := test.adminAnswer(operator, passwordStatus, request.Method, path)
	writer.WriteHeader(status)
	writer.Write([]byte(body))
}

func (test *L3PeopleChangeTest) adminAnswer(operator bool, passwordStatus int, method string, path string) (int, string) {
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

func (test *L3PeopleChangeTest) keySet(keys oidc.KeySet) []byte {
	encoded, err := json.Marshal(keys)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return encoded
}

func (test *L3PeopleChangeTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
