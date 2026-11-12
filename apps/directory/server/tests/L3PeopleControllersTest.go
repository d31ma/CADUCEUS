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
type L3PeopleControllersTest struct {
	behaviors *BehaviorContext
}

func (test *L3PeopleControllersTest) TestGetPeopleGivesAnOperatorAPage() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	tag := test.unique()
	person := test.stored(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + tag, State: "active"})
	defer test.forget([]string{}, person)
	request := test.request("/api/v1/directory/people?query="+tag+"&limit=10", test.bearer(realm, "operator-"+test.unique(), "*", "mesail-operator"))
	page, err := test.peopleController().GetPeople(request)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(1, len(page.Items))
	Assert.Equals(person.Username, page.Items[0].Username)
}

func (test *L3PeopleControllersTest) TestGetPeopleRefusesADomainAdmin() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	request := test.request("/api/v1/directory/people", test.bearer(realm, "admin-"+test.unique(), "example.com", "mesail-admin"))
	_, err := test.peopleController().GetPeople(request)
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
}

func (test *L3PeopleControllersTest) TestGetPeopleRefusesALimitThatIsNotANumber() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	request := test.request("/api/v1/directory/people?limit=many", test.bearer(realm, "operator-"+test.unique(), "*", "mesail-operator"))
	_, err := test.peopleController().GetPeople(request)
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("limit", invalid.Field)
}

func (test *L3PeopleControllersTest) TestGetPersonGivesAnInstallWideServiceTheRecord() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	record, err := test.peopleController().GetPerson(DirectoryTestSetup{}.ServiceBearer(realm, "*"), person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(person.Username, record.Username)
	Assert.Equals(domainId, record.DefaultDomainId)
}

func (test *L3PeopleControllersTest) TestGetPersonHidesThePersonFromAMailDomainCaller() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	_, err := test.peopleController().GetPerson(DirectoryTestSetup{}.ServiceBearer(realm, "example.com"), "anyone")
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3PeopleControllersTest) TestPatchPersonRenamesForAnOperator() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	summary, err := test.peopleController().PatchPerson(test.bearer(realm, "operator-"+test.unique(), "*", "mesail-operator"), person.PersonId, PersonChange{DisplayName: "Renamed"})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("Renamed", summary.DisplayName)
}

func (test *L3PeopleControllersTest) TestPostPasswordResetsThePasswordForAnOperator() {
	realm := test.realm()
	base, calls, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.peopleController().PostPassword(test.bearer(realm, "operator-"+test.unique(), "*", "mesail-operator"), person.PersonId, PasswordReset{NewPassword: "a-long-new-password"})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(strings.Contains(calls(), "/reset-password"), "want a password reset")
}

func (test *L3PeopleControllersTest) TestDeleteTotpRemovesTheCredentialForAnOperator() {
	realm := test.realm()
	base, calls, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.peopleController().DeleteTotp(test.bearer(realm, "operator-"+test.unique(), "*", "mesail-operator"), person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(strings.Contains(calls(), "/credentials/otp-1"), "want the one-time password removed")
}

func (test *L3PeopleControllersTest) TestDeletePersonDeletesForAnOperator() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.peopleController().DeletePerson(test.bearer(realm, "operator-"+test.unique(), "*", "mesail-operator"), person.PersonId)
	stored, _, _ := test.people().ByPersonId(person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("deleted", stored.State)
}

func (test *L3PeopleControllersTest) TestDeletePersonRefusesADomainAdmin() {
	realm := test.realm()
	base, _, closeServer := test.keycloak(false, http.StatusNoContent)
	defer closeServer()
	test.configure(realm, base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	err := test.peopleController().DeletePerson(test.bearer(realm, "admin-"+test.unique(), domainId, "mesail-admin"), person.PersonId)
	stored, _, _ := test.people().ByPersonId(person.PersonId)
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
	Assert.Equals("active", stored.State)
}

func (test *L3PeopleControllersTest) TestPeopleRoutesRefuseARequestWithoutAValidToken() {
	controller := test.peopleController()
	_, listErr := controller.GetPeople(test.request("/api/v1/directory/people", "Bearer forged"))
	_, readErr := controller.GetPerson("Bearer forged", "anyone")
	_, changeErr := controller.PatchPerson("Bearer forged", "anyone", PersonChange{DisplayName: "Evil"})
	passwordErr := controller.PostPassword("Bearer forged", "anyone", PasswordReset{NewPassword: "a-long-new-password"})
	totpErr := controller.DeleteTotp("Bearer forged", "anyone")
	deleteErr := controller.DeletePerson("Bearer forged", "anyone")
	Assert.IsTrue(test.unauthenticated(listErr), "list: want Unauthenticated")
	Assert.IsTrue(test.unauthenticated(readErr), "read: want Unauthenticated")
	Assert.IsTrue(test.unauthenticated(changeErr), "change: want Unauthenticated")
	Assert.IsTrue(test.unauthenticated(passwordErr), "password: want Unauthenticated")
	Assert.IsTrue(test.unauthenticated(totpErr), "totp: want Unauthenticated")
	Assert.IsTrue(test.unauthenticated(deleteErr), "delete: want Unauthenticated")
}

func (test *L3PeopleControllersTest) peopleController() *PeopleController {
	return &PeopleController{callers: &CallerService{}, people: test.personService()}
}

func (test *L3PeopleControllersTest) request(target string, authorization string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Authorization", authorization)
	return request
}

func (test *L3PeopleControllersTest) unauthenticated(err error) bool {
	var refused *Unauthenticated
	return errors.As(err, &refused)
}

func (test *L3PeopleControllersTest) personService() *PersonService {
	return &PersonService{people: test.people(), access: test.access(), accounts: test.accounts(), writer: test.writer(), domains: test.domains()}
}

func (test *L3PeopleControllersTest) access() *PersonAccessService {
	return &PersonAccessService{people: test.people(), accounts: test.accounts()}
}

func (test *L3PeopleControllersTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *L3PeopleControllersTest) writer() *MemberWriterService {
	return &MemberWriterService{members: test.memberStorage(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: test.relay()}
}

func (test *L3PeopleControllersTest) relay() *OutboxRelayService {
	store := &OutboxStoreService{domains: test.domains(), members: test.memberStorage(), names: &CollectionNamesService{}}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
}

func (test *L3PeopleControllersTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleControllersTest) memberStorage() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleControllersTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleControllersTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleControllersTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3PeopleControllersTest) hosted(state string) string {
	domainId := "l3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: state})))
	return domainId
}

func (test *L3PeopleControllersTest) member(domainIds ...string) PersonDocument {
	return test.joined(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Person " + test.unique(), State: "active"}, false, domainIds)
}

func (test *L3PeopleControllersTest) joined(person PersonDocument, admin bool, domainIds []string) PersonDocument {
	for _, domainId := range domainIds {
		person = PersonRules{}.WithAdmin(PersonRules{}.With(person, domainId), domainId, admin)
		_, err := test.writer().Join(person, domainId, admin)
		Assert.Equals("<nil>", fmt.Sprint(err))
	}
	return test.stored(person)
}

func (test *L3PeopleControllersTest) stored(person PersonDocument) PersonDocument {
	saved, err := test.people().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return saved
}

func (test *L3PeopleControllersTest) forget(domainIds []string, people ...PersonDocument) {
	for _, person := range people {
		test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
	}
	for _, domainId := range domainIds {
		domain, _, _ := test.domains().Find(domainId)
		test.domains().Delete(domain)
		(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
	}
}

func (test *L3PeopleControllersTest) realm() oidc.SigningRealm {
	return DirectoryTestSetup{}.SignedRealm()
}

func (test *L3PeopleControllersTest) bearer(realm oidc.SigningRealm, subject string, domainId string, realmRoles ...string) string {
	return DirectoryTestSetup{}.PersonBearer(realm, subject, domainId, realmRoles...)
}

func (test *L3PeopleControllersTest) configure(realm oidc.SigningRealm, base string) {
	os.Setenv("YON_APPLICATION_JSON", DirectoryTestSetup{}.ApplicationJson(realm, base))
}

func (test *L3PeopleControllersTest) keycloak(operator bool, passwordStatus int) (string, func() string, func()) {
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

func (test *L3PeopleControllersTest) answer(operator bool, passwordStatus int, record func(string), writer http.ResponseWriter, request *http.Request) {
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

func (test *L3PeopleControllersTest) adminAnswer(operator bool, passwordStatus int, method string, path string) (int, string) {
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

func (test *L3PeopleControllersTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
