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
type L3MembershipChangeTest struct {
	behaviors *BehaviorContext
}

func (test *L3MembershipChangeTest) TestChangeMakesAMemberAnAdmin() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.hosted()
	defer test.dropDomain(domain)
	added, _ := test.add(test.operator(domain), domain, test.username(), false)
	summary, err := test.membership().Change(test.operator(domain), added.Member.MemberId, MemberChange{Admin: true})
	person, _, _ := test.people().ByPersonId(added.Member.MemberId)
	member, _, _ := test.members().Find(domain, added.Member.MemberId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(summary.Admin, "want an admin summary")
	Assert.IsTrue(member.Admin, "want an admin member")
	Assert.Equals(domain, strings.Join(person.AdminDomainIds, ","))
}

func (test *L3MembershipChangeTest) TestChangeThatRemovesTheLastAdminIsRefused() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.hosted()
	defer test.dropDomain(domain)
	added, _ := test.add(test.operator(domain), domain, test.username(), true)
	_, err := test.membership().Change(test.operator(domain), added.Member.MemberId, MemberChange{Admin: false})
	person, _, _ := test.people().ByPersonId(added.Member.MemberId)
	var lastAdmin *LastAdmin
	Assert.IsTrue(errors.As(err, &lastAdmin), "want LastAdmin")
	Assert.Equals(domain, strings.Join(person.AdminDomainIds, ","))
}

func (test *L3MembershipChangeTest) TestChangeRemovesAnAdminWhileAnotherAdminStays() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.hosted()
	defer test.dropDomain(domain)
	first, _ := test.add(test.operator(domain), domain, test.username(), true)
	test.add(test.operator(domain), domain, test.username(), true)
	summary, err := test.membership().Change(test.operator(domain), first.Member.MemberId, MemberChange{Admin: false})
	person, _, _ := test.people().ByPersonId(first.Member.MemberId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(!summary.Admin, "want no admin")
	Assert.Equals("", strings.Join(person.AdminDomainIds, ","))
}

func (test *L3MembershipChangeTest) TestChangeOfAnUnknownMemberIsNotFound() {
	_, closeAll := test.keycloak(map[string]int{})
	defer closeAll()
	domain := test.hosted()
	defer test.dropDomain(domain)
	_, err := test.membership().Change(test.operator(domain), "kc-"+test.unique(), MemberChange{Admin: true})
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3MembershipChangeTest) keycloak(overrides map[string]int) (func() string, func()) {
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

func (test *L3MembershipChangeTest) answer(overrides map[string]int, record func(string), writer http.ResponseWriter, request *http.Request) {
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

func (test *L3MembershipChangeTest) users(key string, body []byte, writer http.ResponseWriter, request *http.Request) {
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

func (test *L3MembershipChangeTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *L3MembershipChangeTest) username() string {
	return "u" + test.unique()
}

func (test *L3MembershipChangeTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3MembershipChangeTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipChangeTest) reservations() *ReservationService {
	return &ReservationService{localParts: &LocalPartStorageService{documents: test.documents(), names: &CollectionNamesService{}}}
}

func (test *L3MembershipChangeTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *L3MembershipChangeTest) enroll() *PersonEnrollService {
	return &PersonEnrollService{people: test.people(), reservations: test.reservations(), accounts: test.accounts()}
}

func (test *L3MembershipChangeTest) members() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipChangeTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipChangeTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipChangeTest) writer() *MemberWriterService {
	names := &CollectionNamesService{}
	store := &OutboxStoreService{domains: test.domains(), members: test.members(), names: names}
	relay := &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
	return &MemberWriterService{members: test.members(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: relay}
}

func (test *L3MembershipChangeTest) membership() *MembershipService {
	access := &PersonAccessService{people: test.people(), accounts: test.accounts()}
	return &MembershipService{tenants: &TenantService{domains: test.domains()}, enroll: test.enroll(), people: test.people(), writer: test.writer(), access: access}
}

func (test *L3MembershipChangeTest) hosted() string {
	domainId := "l3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	return domainId
}

func (test *L3MembershipChangeTest) dropDomain(domainId string) {
	domain, _, _ := test.domains().Find(domainId)
	test.domains().Delete(domain)
	(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
}

func (test *L3MembershipChangeTest) operator(domain string) oidc.Caller {
	return oidc.Caller{DomainId: domain, MemberId: "kc-op-" + test.unique(), Roles: []string{"Operator"}}
}

func (test *L3MembershipChangeTest) add(caller oidc.Caller, domain string, username string, admin bool) (AddedMember, error) {
	request := AddMemberRequest{Username: username, DisplayName: "Person " + username, FirstPassword: "first-password", Admin: admin}
	return test.membership().Add(caller, domain, request)
}
