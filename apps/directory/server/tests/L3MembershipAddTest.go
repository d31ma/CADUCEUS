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
type L3MembershipAddTest struct {
	behaviors *BehaviorContext
}

func (test *L3MembershipAddTest) TestAddCreatesThePersonAndTheMember() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.hosted()
	defer test.dropDomain(domain)
	username := test.username()
	request := AddMemberRequest{Username: username, DisplayName: "Ana Admin", FirstPassword: "first-password", Admin: true}
	added, err := test.membership().Add(test.operator(domain), " "+strings.ToUpper(domain)+" ", request)
	person, _, _ := test.people().ByPersonId("kc-" + username)
	route, found, _ := test.addresses().Find(username + "@" + domain)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(!added.ExistingPerson, "want a new person")
	Assert.Equals("kc-"+username, added.Member.MemberId)
	Assert.Equals(username, added.Member.Username)
	Assert.Equals(username+"@"+domain, added.Member.PrimaryAddress)
	Assert.IsTrue(added.Member.Admin, "want an admin")
	Assert.IsTrue(added.Member.AccountManaged, "want the account managed by an operator")
	Assert.Equals(domain, strings.Join(person.DomainIds, ","))
	Assert.Equals(domain, strings.Join(person.AdminDomainIds, ","))
	Assert.Equals(domain, person.DefaultDomainId)
	Assert.IsTrue(found, "primary address not stored")
	Assert.Equals("member", route.Kind)
}

func (test *L3MembershipAddTest) TestADomainAdminSeesTheAccountOfANewMemberAsManaged() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.hosted()
	defer test.dropDomain(domain)
	first, _ := test.add(test.operator(domain), domain, test.username(), true)
	caller := oidc.Caller{DomainId: domain, MemberId: first.Member.MemberId, Roles: []string{"DomainAdmin"}}
	second, err := test.add(caller, domain, test.username(), false)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(!second.Member.Admin, "want no admin")
	Assert.IsTrue(second.Member.AccountManaged, "want the account managed by the domain admin")
}

func (test *L3MembershipAddTest) TestAddToADomainThatIsNotHostedIsForbidden() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := "unknown-" + test.unique() + ".test"
	_, err := test.add(test.operator(domain), domain, test.username(), false)
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
	Assert.Equals("", calls())
}

func (test *L3MembershipAddTest) TestAddWithoutADisplayNameIsInvalid() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.hosted()
	defer test.dropDomain(domain)
	_, err := test.membership().Add(test.operator(domain), domain, AddMemberRequest{Username: test.username(), FirstPassword: "first-password"})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("displayName", invalid.Field)
}

func (test *L3MembershipAddTest) TestAddingTheSameMemberTwiceIsADuplicate() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.hosted()
	defer test.dropDomain(domain)
	username := test.username()
	test.add(test.operator(domain), domain, username, true)
	_, err := test.add(test.operator(domain), domain, username, false)
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *L3MembershipAddTest) TestAddingAKnownPersonToASecondDomainReusesThePerson() {
	calls, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	first, second := test.hosted(), test.hosted()
	defer test.dropDomain(first)
	defer test.dropDomain(second)
	username := test.username()
	test.add(test.operator(first), first, username, true)
	added, err := test.add(test.operator(second), second, username, false)
	person, _, _ := test.people().ByPersonId("kc-" + username)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(added.ExistingPerson, "want the existing person")
	Assert.Equals(username+"@"+second, added.Member.PrimaryAddress)
	Assert.Equals(first+","+second, strings.Join(person.DomainIds, ","))
	Assert.Equals(first, strings.Join(person.AdminDomainIds, ","))
	Assert.Equals(first, person.DefaultDomainId)
	Assert.Equals(1, strings.Count(calls(), "POST /users"))
}

func (test *L3MembershipAddTest) keycloak(overrides map[string]int) (func() string, func()) {
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

func (test *L3MembershipAddTest) answer(overrides map[string]int, record func(string), writer http.ResponseWriter, request *http.Request) {
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

func (test *L3MembershipAddTest) users(key string, body []byte, writer http.ResponseWriter, request *http.Request) {
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

func (test *L3MembershipAddTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *L3MembershipAddTest) username() string {
	return "u" + test.unique()
}

func (test *L3MembershipAddTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3MembershipAddTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipAddTest) reservations() *ReservationService {
	return &ReservationService{localParts: &LocalPartStorageService{documents: test.documents(), names: &CollectionNamesService{}}}
}

func (test *L3MembershipAddTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *L3MembershipAddTest) enroll() *PersonEnrollService {
	return &PersonEnrollService{people: test.people(), reservations: test.reservations(), accounts: test.accounts()}
}

func (test *L3MembershipAddTest) members() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipAddTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipAddTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipAddTest) writer() *MemberWriterService {
	names := &CollectionNamesService{}
	store := &OutboxStoreService{domains: test.domains(), members: test.members(), names: names}
	relay := &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
	return &MemberWriterService{members: test.members(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: relay}
}

func (test *L3MembershipAddTest) membership() *MembershipService {
	access := &PersonAccessService{people: test.people(), accounts: test.accounts()}
	return &MembershipService{tenants: &TenantService{domains: test.domains()}, enroll: test.enroll(), people: test.people(), writer: test.writer(), access: access}
}

func (test *L3MembershipAddTest) hosted() string {
	domainId := "l3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	return domainId
}

func (test *L3MembershipAddTest) dropDomain(domainId string) {
	domain, _, _ := test.domains().Find(domainId)
	test.domains().Delete(domain)
	(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
}

func (test *L3MembershipAddTest) operator(domain string) oidc.Caller {
	return oidc.Caller{DomainId: domain, MemberId: "kc-op-" + test.unique(), Roles: []string{"Operator"}}
}

func (test *L3MembershipAddTest) add(caller oidc.Caller, domain string, username string, admin bool) (AddedMember, error) {
	request := AddMemberRequest{Username: username, DisplayName: "Person " + username, FirstPassword: "first-password", Admin: admin}
	return test.membership().Add(caller, domain, request)
}
