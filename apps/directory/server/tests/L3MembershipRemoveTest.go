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
type L3MembershipRemoveTest struct {
	behaviors *BehaviorContext
}

func (test *L3MembershipRemoveTest) TestRemoveTakesTheMemberOutOfTheDomain() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.hosted()
	defer test.dropDomain(domain)
	test.add(test.operator(domain), domain, test.username(), true)
	member, _ := test.add(test.operator(domain), domain, test.username(), false)
	err := test.membership().Remove(test.operator(domain), member.Member.MemberId)
	_, activeErr := test.writer().Active(domain, member.Member.MemberId)
	person, _, _ := test.people().ByPersonId(member.Member.MemberId)
	_, found, _ := test.addresses().Find(member.Member.PrimaryAddress)
	var missing *NotFound
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(errors.As(activeErr, &missing), "want the member gone")
	Assert.Equals("", strings.Join(person.DomainIds, ","))
	Assert.Equals("", person.DefaultDomainId)
	Assert.IsTrue(!found, "primary address still stored")
}

func (test *L3MembershipRemoveTest) TestRemovingTheLastAdminIsRefused() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.hosted()
	defer test.dropDomain(domain)
	added, _ := test.add(test.operator(domain), domain, test.username(), true)
	err := test.membership().Remove(test.operator(domain), added.Member.MemberId)
	var lastAdmin *LastAdmin
	Assert.IsTrue(errors.As(err, &lastAdmin), "want LastAdmin")
}

func (test *L3MembershipRemoveTest) TestRemovingAnUnknownMemberIsNotFound() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.hosted()
	defer test.dropDomain(domain)
	err := test.membership().Remove(test.operator(domain), "kc-"+test.unique())
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3MembershipRemoveTest) keycloak(overrides map[string]int) (func() string, func()) {
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

func (test *L3MembershipRemoveTest) answer(overrides map[string]int, record func(string), writer http.ResponseWriter, request *http.Request) {
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

func (test *L3MembershipRemoveTest) users(key string, body []byte, writer http.ResponseWriter, request *http.Request) {
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

func (test *L3MembershipRemoveTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *L3MembershipRemoveTest) username() string {
	return "u" + test.unique()
}

func (test *L3MembershipRemoveTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3MembershipRemoveTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipRemoveTest) reservations() *ReservationService {
	return &ReservationService{localParts: &LocalPartStorageService{documents: test.documents(), names: &CollectionNamesService{}}}
}

func (test *L3MembershipRemoveTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *L3MembershipRemoveTest) enroll() *PersonEnrollService {
	return &PersonEnrollService{people: test.people(), reservations: test.reservations(), accounts: test.accounts()}
}

func (test *L3MembershipRemoveTest) members() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipRemoveTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipRemoveTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipRemoveTest) writer() *MemberWriterService {
	names := &CollectionNamesService{}
	store := &OutboxStoreService{domains: test.domains(), members: test.members(), names: names}
	relay := &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
	return &MemberWriterService{members: test.members(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: relay}
}

func (test *L3MembershipRemoveTest) membership() *MembershipService {
	access := &PersonAccessService{people: test.people(), accounts: test.accounts()}
	return &MembershipService{tenants: &TenantService{domains: test.domains()}, enroll: test.enroll(), people: test.people(), writer: test.writer(), access: access}
}

func (test *L3MembershipRemoveTest) hosted() string {
	domainId := "l3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	return domainId
}

func (test *L3MembershipRemoveTest) dropDomain(domainId string) {
	domain, _, _ := test.domains().Find(domainId)
	test.domains().Delete(domain)
	(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
}

func (test *L3MembershipRemoveTest) operator(domain string) oidc.Caller {
	return oidc.Caller{DomainId: domain, MemberId: "kc-op-" + test.unique(), Roles: []string{"Operator"}}
}

func (test *L3MembershipRemoveTest) add(caller oidc.Caller, domain string, username string, admin bool) (AddedMember, error) {
	request := AddMemberRequest{Username: username, DisplayName: "Person " + username, FirstPassword: "first-password", Admin: admin}
	return test.membership().Add(caller, domain, request)
}
