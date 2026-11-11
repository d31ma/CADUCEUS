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
type L3PeopleAccountTest struct {
	behaviors *BehaviorContext
}

func (test *L3PeopleAccountTest) TestResetPasswordSetsThePasswordAndSignsThePersonOut() {
	base, calls, closeServer := test.keycloak(test.key(), false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.personService().ResetPassword(test.operator(), person.PersonId, PasswordReset{NewPassword: "a-long-new-password"})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(strings.Contains(calls(), "PUT /idp/admin/realms/mesail/users/"+person.PersonId+"/reset-password"), "want a password reset")
	Assert.IsTrue(strings.Contains(calls(), "POST /idp/admin/realms/mesail/users/"+person.PersonId+"/logout"), "want a sign out")
}

func (test *L3PeopleAccountTest) TestAnEmptyPasswordIsRefused() {
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.personService().ResetPassword(test.operator(), person.PersonId, PasswordReset{NewPassword: "   "})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("newPassword", invalid.Field)
}

func (test *L3PeopleAccountTest) TestAPasswordTheIdentityServerRefusesBreaksThePasswordPolicy() {
	base, _, closeServer := test.keycloak(test.key(), false, http.StatusBadRequest)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.personService().ResetPassword(test.operator(), person.PersonId, PasswordReset{NewPassword: "short"})
	var policy *PasswordPolicy
	Assert.IsTrue(errors.As(err, &policy), "want PasswordPolicy")
	Assert.Equals("the password is too short", policy.Message)
}

func (test *L3PeopleAccountTest) TestResetPasswordOfAnUnknownPersonIsNotFound() {
	err := test.personService().ResetPassword(test.operator(), "nobody-"+test.unique(), PasswordReset{NewPassword: "a-long-new-password"})
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3PeopleAccountTest) TestRemoveTotpDeletesOnlyTheOneTimePasswordCredential() {
	base, calls, closeServer := test.keycloak(test.key(), false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.personService().RemoveTotp(test.operator(), person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(strings.Contains(calls(), "DELETE /idp/admin/realms/mesail/users/"+person.PersonId+"/credentials/otp-1"), "want the one-time password removed")
	Assert.IsTrue(!strings.Contains(calls(), "password-1"), "the password must stay")
}

func (test *L3PeopleAccountTest) TestRemoveTotpForAnAdminOfAnotherDomainIsForbidden() {
	first, second := test.hosted("active"), test.hosted("active")
	admin := test.admin(second)
	person := test.member(first)
	defer test.forget([]string{first, second}, admin, person)
	err := test.personService().RemoveTotp(test.domainAdmin(admin, second), person.PersonId)
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
}

func (test *L3PeopleAccountTest) TestDeleteRemovesTheAccountAndEveryMembership() {
	base, calls, closeServer := test.keycloak(test.key(), false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	first, second := test.hosted("active"), test.hosted("active")
	person := test.member(first, second)
	defer test.forget([]string{first, second}, person)
	err := test.personService().Delete(person.PersonId)
	stored, _, _ := test.people().ByPersonId(person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(strings.Contains(calls(), "DELETE /idp/admin/realms/mesail/users/"+person.PersonId), "want the account deleted")
	Assert.Equals("removed", test.memberDocument(first, person.PersonId).State)
	Assert.Equals("removed", test.memberDocument(second, person.PersonId).State)
	Assert.Equals("deleted", stored.State)
	Assert.Equals(0, len(stored.DomainIds))
	Assert.Equals("", stored.DefaultDomainId)
	Assert.Equals(person.Username, stored.Username)
}

func (test *L3PeopleAccountTest) TestDeleteOfAnUnknownPersonIsNotFound() {
	err := test.personService().Delete("nobody-" + test.unique())
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3PeopleAccountTest) TestDeleteKeepsThePersonWhenTheIdentityServerFails() {
	test.configureWithoutKeycloak()
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.personService().Delete(person.PersonId)
	stored, _, _ := test.people().ByPersonId(person.PersonId)
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
	Assert.Equals("active", stored.State)
	Assert.Equals("active", test.memberDocument(domainId, person.PersonId).State)
}

func (test *L3PeopleAccountTest) TestSetDefaultChoosesADomainOfTheCaller() {
	first, second := test.hosted("active"), test.hosted("active")
	person := test.member(first, second)
	defer test.forget([]string{first, second}, person)
	err := test.personService().SetDefault(oidc.Caller{MemberId: person.PersonId, DomainId: first}, DefaultDomain{DomainId: "  " + strings.ToUpper(second) + " "})
	stored, _, _ := test.people().ByPersonId(person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(second, stored.DefaultDomainId)
}

func (test *L3PeopleAccountTest) TestSetDefaultToADomainOfSomeoneElseIsNotFound() {
	first, second := test.hosted("active"), test.hosted("active")
	person := test.member(first)
	defer test.forget([]string{first, second}, person)
	err := test.personService().SetDefault(oidc.Caller{MemberId: person.PersonId, DomainId: first}, DefaultDomain{DomainId: second})
	stored, _, _ := test.people().ByPersonId(person.PersonId)
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
	Assert.Equals(first, stored.DefaultDomainId)
}

func (test *L3PeopleAccountTest) TestSetDefaultForACallerWithoutAPersonIsNotFound() {
	err := test.personService().SetDefault(oidc.Caller{MemberId: "nobody-" + test.unique()}, DefaultDomain{DomainId: "example.com"})
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3PeopleAccountTest) personService() *PersonService {
	return &PersonService{people: test.people(), access: test.access(), accounts: test.accounts(), writer: test.writer()}
}

func (test *L3PeopleAccountTest) access() *PersonAccessService {
	return &PersonAccessService{people: test.people(), accounts: test.accounts()}
}

func (test *L3PeopleAccountTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *L3PeopleAccountTest) writer() *MemberWriterService {
	return &MemberWriterService{members: test.memberStorage(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: test.relay()}
}

func (test *L3PeopleAccountTest) relay() *OutboxRelayService {
	store := &OutboxStoreService{domains: test.domains(), members: test.memberStorage(), names: &CollectionNamesService{}}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
}

func (test *L3PeopleAccountTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleAccountTest) memberStorage() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleAccountTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleAccountTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleAccountTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3PeopleAccountTest) hosted(state string) string {
	domainId := "l3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: state})))
	return domainId
}

func (test *L3PeopleAccountTest) member(domainIds ...string) PersonDocument {
	return test.joined(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Person " + test.unique(), State: "active"}, false, domainIds)
}

func (test *L3PeopleAccountTest) admin(domainIds ...string) PersonDocument {
	return test.joined(PersonDocument{PersonId: "a-" + test.unique(), Username: "a" + test.unique(), DisplayName: "Admin " + test.unique(), State: "active"}, true, domainIds)
}

func (test *L3PeopleAccountTest) joined(person PersonDocument, admin bool, domainIds []string) PersonDocument {
	for _, domainId := range domainIds {
		person = PersonRules{}.WithAdmin(PersonRules{}.With(person, domainId), domainId, admin)
		_, err := test.writer().Join(person, domainId, admin)
		Assert.Equals("<nil>", fmt.Sprint(err))
	}
	return test.stored(person)
}

func (test *L3PeopleAccountTest) stored(person PersonDocument) PersonDocument {
	saved, err := test.people().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return saved
}

func (test *L3PeopleAccountTest) forget(domainIds []string, people ...PersonDocument) {
	for _, person := range people {
		test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
	}
	for _, domainId := range domainIds {
		domain, _, _ := test.domains().Find(domainId)
		test.domains().Delete(domain)
		(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
	}
}

func (test *L3PeopleAccountTest) memberDocument(domainId string, personId string) MemberDocument {
	member, _, _ := test.memberStorage().Find(domainId, personId)
	return member
}

func (test *L3PeopleAccountTest) operator() oidc.Caller {
	return oidc.Caller{DomainId: "*", MemberId: "operator-" + test.unique(), Roles: []string{"Operator"}}
}

func (test *L3PeopleAccountTest) domainAdmin(admin PersonDocument, domainId string) oidc.Caller {
	return oidc.Caller{DomainId: domainId, MemberId: admin.PersonId, Username: admin.Username, Roles: []string{"DomainAdmin"}}
}

func (test *L3PeopleAccountTest) key() oidc.SigningKey {
	key, err := oidc.SigningKey{}.Generated()
	Assert.Equals("<nil>", fmt.Sprint(err))
	return key
}

func (test *L3PeopleAccountTest) configure(base string) {
	settings := `{"yon":{"oidc":{"issuer":%q,"jwks":%q,"audience":%q},"directory":{"keycloakurl":%q,"admin":{"client":"mesail-directory","secret":"example-secret"}}}}`
	os.Setenv("YON_APPLICATION_JSON", fmt.Sprintf(settings, test.issuer(base), base+"/jwks/"+test.unique(), test.audience(), base+"/idp"))
}

func (test *L3PeopleAccountTest) configureWithoutKeycloak() {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{}}`)
}

func (test *L3PeopleAccountTest) issuer(base string) string {
	return base + "/realms/mesail-test"
}

func (test *L3PeopleAccountTest) audience() string {
	return "mesail-directory-test"
}

func (test *L3PeopleAccountTest) keycloak(key oidc.SigningKey, operator bool, passwordStatus int) (string, func() string, func()) {
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

func (test *L3PeopleAccountTest) answer(key oidc.SigningKey, operator bool, passwordStatus int, record func(string), writer http.ResponseWriter, request *http.Request) {
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

func (test *L3PeopleAccountTest) adminAnswer(operator bool, passwordStatus int, method string, path string) (int, string) {
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

func (test *L3PeopleAccountTest) keySet(keys oidc.KeySet) []byte {
	encoded, err := json.Marshal(keys)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return encoded
}

func (test *L3PeopleAccountTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
