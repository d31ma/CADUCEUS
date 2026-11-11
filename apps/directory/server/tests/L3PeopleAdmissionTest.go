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
	"time"

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type L3PeopleAdmissionTest struct {
	behaviors *BehaviorContext
}

func (test *L3PeopleAdmissionTest) TestAnActiveMemberOfAHostedDomainIsAdmitted() {
	key := test.key()
	base, _, closeServer := test.keycloak(key, false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	admission := test.admission().Admit(test.bearer(key, base, person.PersonId, domainId, "mesail-member"))
	Assert.Equals("<nil>", fmt.Sprint(admission.Refusal))
	Assert.Equals(person.PersonId, admission.Caller.MemberId)
	Assert.Equals(domainId, admission.Caller.DomainId)
}

func (test *L3PeopleAdmissionTest) TestAPersonWhoIsNotAMemberOfTheTokenDomainIsForbidden() {
	key := test.key()
	base, _, closeServer := test.keycloak(key, false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	first, second := test.hosted("active"), test.hosted("active")
	person := test.member(first)
	defer test.forget([]string{first, second}, person)
	admission := test.admission().Admit(test.bearer(key, base, person.PersonId, second, "mesail-member"))
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(admission.Refusal, &forbidden), "want Forbidden")
}

func (test *L3PeopleAdmissionTest) TestADisabledPersonIsForbidden() {
	key := test.key()
	base, _, closeServer := test.keycloak(key, false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	person := test.stored(PersonRules{}.With(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), State: "disabled"}, domainId))
	defer test.forget([]string{domainId}, person)
	admission := test.admission().Admit(test.bearer(key, base, person.PersonId, domainId, "mesail-member"))
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(admission.Refusal, &forbidden), "want Forbidden")
}

func (test *L3PeopleAdmissionTest) TestAMemberOfADisabledDomainIsForbidden() {
	key := test.key()
	base, _, closeServer := test.keycloak(key, false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("disabled")
	person := test.member(domainId)
	defer test.forget([]string{domainId}, person)
	admission := test.admission().Admit(test.bearer(key, base, person.PersonId, domainId, "mesail-member"))
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(admission.Refusal, &forbidden), "want Forbidden")
}

func (test *L3PeopleAdmissionTest) TestACallerWithoutAPersonIsRememberedFromTheToken() {
	key := test.key()
	base, _, closeServer := test.keycloak(key, false, http.StatusNoContent)
	defer closeServer()
	test.configure(base)
	domainId := test.hosted("active")
	defer test.forget([]string{domainId})
	subject := "token-" + test.unique()
	admission := test.admission().Admit(test.bearer(key, base, subject, domainId, "mesail-member"))
	member, found, _ := test.memberStorage().Find(domainId, subject)
	Assert.Equals("<nil>", fmt.Sprint(admission.Refusal))
	Assert.IsTrue(found, "want the member remembered from the token")
	Assert.Equals("caller@"+domainId, member.PrimaryAddress)
}

func (test *L3PeopleAdmissionTest) TestATokenThatCannotBeVerifiedIsUnauthenticated() {
	admission := test.admission().Admit("Bearer forged")
	var refused *Unauthenticated
	Assert.IsTrue(errors.As(admission.Refusal, &refused), "want Unauthenticated")
}

func (test *L3PeopleAdmissionTest) admission() *AdmissionService {
	tenants := &TenantService{domains: test.domains()}
	tokenMembers := &TokenMemberService{tenants: tenants, members: test.memberStorage(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: test.relay()}
	return &AdmissionService{callers: &CallerService{}, tokenMembers: tokenMembers, people: test.people(), tenants: tenants}
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

func (test *L3PeopleAdmissionTest) key() oidc.SigningKey {
	key, err := oidc.SigningKey{}.Generated()
	Assert.Equals("<nil>", fmt.Sprint(err))
	return key
}

func (test *L3PeopleAdmissionTest) bearer(key oidc.SigningKey, base string, subject string, domainId string, realmRoles ...string) string {
	signer := oidc.TokenSigner{Issuer: test.issuer(base), Audience: test.audience(), Current: key, Now: time.Now}
	token := oidc.MesailToken{Subject: subject, AuthorizedParty: oidc.AuthorizedParties{}.Web(), Domain: domainId, RealmRoles: realmRoles, Name: "Test Caller", Username: "caller", Email: "caller@" + domainId}
	issued, err := signer.Sign(token)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return "Bearer " + issued.AccessToken
}

func (test *L3PeopleAdmissionTest) configure(base string) {
	settings := `{"yon":{"oidc":{"issuer":%q,"jwks":%q,"audience":%q},"directory":{"keycloakurl":%q,"admin":{"client":"mesail-directory","secret":"example-secret"}}}}`
	os.Setenv("YON_APPLICATION_JSON", fmt.Sprintf(settings, test.issuer(base), base+"/jwks/"+test.unique(), test.audience(), base+"/idp"))
}

func (test *L3PeopleAdmissionTest) issuer(base string) string {
	return base + "/realms/mesail-test"
}

func (test *L3PeopleAdmissionTest) audience() string {
	return "mesail-directory-test"
}

func (test *L3PeopleAdmissionTest) keycloak(key oidc.SigningKey, operator bool, passwordStatus int) (string, func() string, func()) {
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

func (test *L3PeopleAdmissionTest) answer(key oidc.SigningKey, operator bool, passwordStatus int, record func(string), writer http.ResponseWriter, request *http.Request) {
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

func (test *L3PeopleAdmissionTest) keySet(keys oidc.KeySet) []byte {
	encoded, err := json.Marshal(keys)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return encoded
}

func (test *L3PeopleAdmissionTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
