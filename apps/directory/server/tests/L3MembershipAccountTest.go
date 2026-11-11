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
type L3MembershipAccountTest struct {
	behaviors *BehaviorContext
}

func (test *L3MembershipAccountTest) TestCreateReturnsTheIdFromTheLocationHeader() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	id, err := test.accounts().Create("ana", "first-password")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("kc-ana", id)
	Assert.Equals("POST /users", calls())
}

func (test *L3MembershipAccountTest) TestCreateOfATakenUsernameIsADuplicate() {
	_, closeAll := test.keycloak(map[string]int{"POST /users": http.StatusConflict})
	defer closeAll()
	_, err := test.accounts().Create("ana", "first-password")
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *L3MembershipAccountTest) TestCreateWithAPasswordThePolicyRefusesIsAPasswordPolicyFailure() {
	_, closeAll := test.keycloak(map[string]int{"POST /users": http.StatusBadRequest})
	defer closeAll()
	_, err := test.accounts().Create("ana", "short")
	var policy *PasswordPolicy
	Assert.IsTrue(errors.As(err, &policy), "want PasswordPolicy")
	Assert.IsTrue(strings.Contains(policy.Message, "refused by the fake"), "want the reason of Keycloak")
}

func (test *L3MembershipAccountTest) TestCreateWhenKeycloakFailsIsUnavailable() {
	_, closeAll := test.keycloak(map[string]int{"POST /users": http.StatusInternalServerError})
	defer closeAll()
	_, err := test.accounts().Create("ana", "first-password")
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
}

func (test *L3MembershipAccountTest) TestCreateWithoutKeycloakSettingsIsUnavailable() {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{}}`)
	_, err := test.accounts().Create("ana", "first-password")
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
}

func (test *L3MembershipAccountTest) TestSetEnabledReadsAndWritesTheUser() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	err := test.accounts().SetEnabled("kc-ana", false)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("GET /users/kc-ana\nPUT /users/kc-ana", calls())
}

func (test *L3MembershipAccountTest) TestSetEnabledOfAMissingAccountIsNotFound() {
	_, closeAll := test.keycloak(map[string]int{"GET /users/kc-gone": http.StatusNotFound})
	defer closeAll()
	err := test.accounts().SetEnabled("kc-gone", true)
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3MembershipAccountTest) TestResetPasswordSetsThePasswordAndSignsTheUserOut() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	err := test.accounts().ResetPassword("kc-ana", "new-password")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("PUT /users/kc-ana/reset-password\nGET /users/kc-ana\nPUT /users/kc-ana\nPOST /users/kc-ana/logout", calls())
}

func (test *L3MembershipAccountTest) TestResetPasswordThePolicyRefusesIsAPasswordPolicyFailure() {
	_, closeAll := test.keycloak(map[string]int{"PUT /users/kc-ana/reset-password": http.StatusBadRequest})
	defer closeAll()
	err := test.accounts().ResetPassword("kc-ana", "short")
	var policy *PasswordPolicy
	Assert.IsTrue(errors.As(err, &policy), "want PasswordPolicy")
}

func (test *L3MembershipAccountTest) TestRemoveTotpDeletesOnlyTheOneTimePasswordCredential() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	err := test.accounts().RemoveTotp("kc-ana")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("GET /users/kc-ana/credentials\nDELETE /users/kc-ana/credentials/otp-1", calls())
}

func (test *L3MembershipAccountTest) TestRemoveTotpOfAMissingAccountIsNotFound() {
	_, closeAll := test.keycloak(map[string]int{"GET /users/kc-gone/credentials": http.StatusNotFound})
	defer closeAll()
	err := test.accounts().RemoveTotp("kc-gone")
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3MembershipAccountTest) TestDeleteRemovesTheAccount() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	err := test.accounts().Delete("kc-ana")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("DELETE /users/kc-ana", calls())
}

func (test *L3MembershipAccountTest) TestDeleteOfAnAccountThatIsAlreadyGoneSucceeds() {
	_, closeAll := test.keycloak(map[string]int{"DELETE /users/kc-gone": http.StatusNotFound})
	defer closeAll()
	err := test.accounts().Delete("kc-gone")
	Assert.Equals("<nil>", fmt.Sprint(err))
}

func (test *L3MembershipAccountTest) TestDeleteWhenKeycloakFailsIsUnavailable() {
	_, closeAll := test.keycloak(map[string]int{"DELETE /users/kc-ana": http.StatusInternalServerError})
	defer closeAll()
	err := test.accounts().Delete("kc-ana")
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
}

func (test *L3MembershipAccountTest) TestIsOperatorIsTrueForTheOperatorRole() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	operator, err := test.accounts().IsOperator("kc-op-ana")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(operator, "want an operator")
}

func (test *L3MembershipAccountTest) TestIsOperatorIsFalseWithoutTheOperatorRole() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	operator, err := test.accounts().IsOperator("kc-ana")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(!operator, "want no operator")
}

func (test *L3MembershipAccountTest) TestIsOperatorWhenKeycloakRefusesIsUnavailable() {
	_, closeAll := test.keycloak(map[string]int{"GET /users/kc-ana/role-mappings/realm/composite": http.StatusForbidden})
	defer closeAll()
	_, err := test.accounts().IsOperator("kc-ana")
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
}

func (test *L3MembershipAccountTest) TestManagerOfAnOperatorCallerIsAnOperator() {
	manager, err := test.access().ManagerOf(test.operator())
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(manager.Operator, "want an operator")
	Assert.Equals(0, len(manager.AdminDomainIds))
}

func (test *L3MembershipAccountTest) TestManagerOfADomainAdminHoldsTheAdminDomainsOfThatPerson() {
	domain := test.domain()
	manager, err := test.access().ManagerOf(test.admin(domain))
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(!manager.Operator, "want no operator")
	Assert.Equals(domain, strings.Join(manager.AdminDomainIds, ","))
}

func (test *L3MembershipAccountTest) TestAnOperatorMayChangeTheAccountOfAnotherOperator() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	person := PersonDocument{PersonId: "kc-op-" + test.unique(), DomainIds: []string{test.domain()}}
	err := test.access().Check(test.operator(), person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("", calls())
}

func (test *L3MembershipAccountTest) TestAnOperatorMayChangeTheAccountOfAPersonWithoutDomains() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	err := test.access().Check(test.operator(), PersonDocument{PersonId: "kc-" + test.unique()})
	Assert.Equals("<nil>", fmt.Sprint(err))
}

func (test *L3MembershipAccountTest) TestAnAdminOfEveryDomainOfThePersonMayChangeTheAccount() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	first, second := test.domain(), test.domain()
	personId := "kc-" + test.unique()
	err := test.access().Check(test.admin(first, second), PersonDocument{PersonId: personId, DomainIds: []string{first, second}})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("GET /users/"+personId+"/role-mappings/realm/composite", calls())
}

func (test *L3MembershipAccountTest) TestAnAdminOfOnlySomeDomainsOfThePersonIsForbidden() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	first, second := test.domain(), test.domain()
	err := test.access().Check(test.admin(first), PersonDocument{PersonId: "kc-" + test.unique(), DomainIds: []string{first, second}})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
	Assert.Equals("", calls())
}

func (test *L3MembershipAccountTest) TestADomainAdminMayNotChangeTheAccountOfAnOperator() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.domain()
	err := test.access().Check(test.admin(domain), PersonDocument{PersonId: "kc-op-" + test.unique(), DomainIds: []string{domain}})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
}

func (test *L3MembershipAccountTest) TestAPersonWithNoDomainsIsNotManagedByADomainAdmin() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	err := test.access().Check(test.admin(test.domain()), PersonDocument{PersonId: "kc-" + test.unique(), DomainIds: []string{}})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
}

func (test *L3MembershipAccountTest) TestACallerWithoutAPersonRecordManagesNothing() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	caller := oidc.Caller{MemberId: "kc-" + test.unique()}
	err := test.access().Check(caller, PersonDocument{PersonId: "kc-" + test.unique(), DomainIds: []string{test.domain()}})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
}

func (test *L3MembershipAccountTest) TestWhenKeycloakCannotTellTheRolesTheCheckIsUnavailable() {
	personId := "kc-" + test.unique()
	_, closeAll := test.keycloak(map[string]int{"GET /users/" + personId + "/role-mappings/realm/composite": http.StatusInternalServerError})
	defer closeAll()
	domain := test.domain()
	err := test.access().Check(test.admin(domain), PersonDocument{PersonId: personId, DomainIds: []string{domain}})
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
}

func (test *L3MembershipAccountTest) keycloak(overrides map[string]int) (func() string, func()) {
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
	test.configure(server.URL + "/idp")
	return calls, server.Close
}

func (test *L3MembershipAccountTest) configure(keycloakUrl string) {
	settings := map[string]any{"yon": map[string]any{"directory": map[string]any{
		"keycloakurl": keycloakUrl, "admin": map[string]any{"client": "mesail-directory", "secret": "example-secret"},
	}}}
	encoded, _ := json.Marshal(settings)
	os.Setenv("YON_APPLICATION_JSON", string(encoded))
}

func (test *L3MembershipAccountTest) answer(overrides map[string]int, record func(string), writer http.ResponseWriter, request *http.Request) {
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
		writer.Write([]byte(`{"errorMessage":"refused by the fake"}`))
		return
	}
	test.normal(key, body, writer, request)
}

func (test *L3MembershipAccountTest) normal(key string, body []byte, writer http.ResponseWriter, request *http.Request) {
	switch {
	case key == "POST /users":
		test.created(body, writer, request)
	case key == "GET /users":
		test.search(request.URL.Query().Get("username"), writer)
	case strings.HasSuffix(key, "/role-mappings/realm/composite"):
		test.roles(key, writer)
	case strings.HasSuffix(key, "/credentials"):
		writer.Write([]byte(`[{"id":"otp-1","type":"otp"},{"id":"pw-1","type":"password"}]`))
	case request.Method == http.MethodGet:
		writer.Write([]byte(`{"enabled":true,"attributes":{}}`))
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}

func (test *L3MembershipAccountTest) created(body []byte, writer http.ResponseWriter, request *http.Request) {
	user := struct {
		Username string `json:"username"`
	}{}
	json.Unmarshal(body, &user)
	writer.Header().Set("Location", "http://"+request.Host+"/idp/admin/realms/mesail/users/kc-"+user.Username)
	writer.WriteHeader(http.StatusCreated)
}

func (test *L3MembershipAccountTest) search(username string, writer http.ResponseWriter) {
	encoded, _ := json.Marshal([]map[string]string{{"id": "kc-" + username, "username": username}})
	writer.Write(encoded)
}

func (test *L3MembershipAccountTest) roles(key string, writer http.ResponseWriter) {
	if strings.HasPrefix(key, "GET /users/kc-op") {
		writer.Write([]byte(`[{"name":"mesail-operator"},{"name":"default-roles-mesail"}]`))
		return
	}
	writer.Write([]byte(`[{"name":"default-roles-mesail"}]`))
}

func (test *L3MembershipAccountTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *L3MembershipAccountTest) username() string {
	return "u" + test.unique()
}

func (test *L3MembershipAccountTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3MembershipAccountTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipAccountTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *L3MembershipAccountTest) access() *PersonAccessService {
	return &PersonAccessService{people: test.people(), accounts: test.accounts()}
}

func (test *L3MembershipAccountTest) domain() string {
	return "a" + test.unique() + ".test"
}

func (test *L3MembershipAccountTest) operator() oidc.Caller {
	return oidc.Caller{MemberId: "kc-" + test.unique(), Roles: []string{"Operator"}}
}

func (test *L3MembershipAccountTest) admin(domainIds ...string) oidc.Caller {
	username := test.username()
	person := PersonDocument{PersonId: "kc-" + username, Username: username, DisplayName: "Domain Admin", DomainIds: domainIds, AdminDomainIds: domainIds, State: "active"}
	_, err := test.people().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return oidc.Caller{MemberId: person.PersonId, Roles: []string{"DomainAdmin"}}
}
