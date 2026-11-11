// tachyon: example secrets
import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type DomainServiceTest struct {
	behaviors *BehaviorContext
}

func (test *DomainServiceTest) TestAddingADomainCreatesItsRealmAndStoresItPending() {
	service, calls, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	name := "Add-" + test.reset("add-unit.test")[4:]
	domain, err := service.Add(CreateDomainRequest{Name: name})
	Assert.IsTrue(err == nil, "add failed")
	Assert.Equals(strings.ToLower(name), domain.DomainId)
	Assert.Equals("pending", domain.State)
	Assert.Equals("mesail", domain.DkimSelector)
	Assert.Equals("missing", domain.Dns.Mx)
	Assert.IsTrue(strings.Contains(calls(), "POST /idp/admin/realms realm="+strings.ToLower(name)), calls())
}

func (test *DomainServiceTest) TestAddingADomainTwiceIsADuplicate() {
	service, _, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	name := test.reset("twice-unit.test")
	service.Add(CreateDomainRequest{Name: name})
	_, err := service.Add(CreateDomainRequest{Name: name})
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *DomainServiceTest) TestARealmThatAlreadyExistsIsAdopted() {
	service, _, closeAll := test.world(http.StatusConflict)
	defer closeAll()
	_, err := service.Add(CreateDomainRequest{Name: test.reset("adopt-unit.test")})
	Assert.IsTrue(err == nil, "add failed")
}

func (test *DomainServiceTest) TestARealmThatKeycloakWillNotCreateButThatExistsIsAdopted() {
	service, calls, closeAll := test.worldWith(http.StatusForbidden, http.StatusNoContent, http.StatusOK)
	defer closeAll()
	name := test.reset("seeded-unit.test")
	domain, err := service.Add(CreateDomainRequest{Name: name})
	Assert.IsTrue(err == nil, "add failed")
	Assert.Equals(name, domain.DomainId)
	Assert.IsTrue(strings.Contains(calls(), "GET /idp/realms/"+name+"/.well-known/openid-configuration"), calls())
}

func (test *DomainServiceTest) TestARealmThatKeycloakWillNotCreateAndThatIsMissingLeavesNoDomain() {
	service, _, closeAll := test.worldWith(http.StatusForbidden, http.StatusNoContent, http.StatusNotFound)
	defer closeAll()
	name := test.reset("refused-unit.test")
	_, err := service.Add(CreateDomainRequest{Name: name})
	_, getErr := service.Get(name)
	var unavailable *Unavailable
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
	Assert.IsTrue(errors.As(getErr, &missing), "want NotFound")
}

func (test *DomainServiceTest) TestAKeycloakFailureLeavesNoDomain() {
	service, _, closeAll := test.world(http.StatusInternalServerError)
	defer closeAll()
	name := test.reset("fail-unit.test")
	_, err := service.Add(CreateDomainRequest{Name: name})
	_, getErr := service.Get(name)
	var unavailable *Unavailable
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
	Assert.IsTrue(errors.As(getErr, &missing), "want NotFound")
}

func (test *DomainServiceTest) TestANameWithoutADotIsRefused() {
	service, _, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	_, err := service.Add(CreateDomainRequest{Name: "mesail-operators"})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("name", invalid.Field)
}

func (test *DomainServiceTest) TestVerifyWithEveryRecordRightMakesTheDomainActive() {
	service, _, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	added, _ := service.Add(CreateDomainRequest{Name: test.reset("good-unit.test")})
	domain, err := service.Verify(added.DomainId)
	stored, _ := service.Get(added.DomainId)
	Assert.IsTrue(err == nil, "verify failed")
	Assert.Equals("active", domain.State)
	Assert.Equals("ok", domain.Dns.Dkim)
	Assert.Equals("active", stored.State)
}

func (test *DomainServiceTest) TestVerifyWithAMissingRecordKeepsTheDomainPending() {
	service, _, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	added, _ := service.Add(CreateDomainRequest{Name: test.reset("partial-unit.test")})
	domain, err := service.Verify(added.DomainId)
	Assert.IsTrue(err == nil, "verify failed")
	Assert.Equals("pending", domain.State)
	Assert.Equals("missing", domain.Dns.Dmarc)
	Assert.Equals("ok", domain.Dns.Mx)
}

func (test *DomainServiceTest) TestVerifyOfAnUnknownDomainIsNotFound() {
	service, _, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	_, err := service.Verify("unknown-" + test.unique() + ".test")
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *DomainServiceTest) TestListShowsAnAddedDomain() {
	service, _, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	added, _ := service.Add(CreateDomainRequest{Name: test.reset("list-unit.test")})
	domains, err := service.List()
	encoded, _ := json.Marshal(domains)
	Assert.IsTrue(err == nil, "list failed")
	Assert.IsTrue(strings.Contains(string(encoded), `"domainId":"`+added.DomainId+`"`), "domain not listed")
}

func (test *DomainServiceTest) TestRemovingADomainDeletesItsRealmAndRecord() {
	service, calls, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	added, _ := service.Add(CreateDomainRequest{Name: test.reset("remove-unit.test")})
	err := service.Remove(added.DomainId)
	_, getErr := service.Get(added.DomainId)
	var missing *NotFound
	Assert.IsTrue(err == nil, "remove failed")
	Assert.IsTrue(errors.As(getErr, &missing), "want NotFound")
	Assert.IsTrue(strings.Contains(calls(), "DELETE /idp/admin/realms/"+added.DomainId), calls())
}

func (test *DomainServiceTest) TestRemovingADomainWhoseRealmKeycloakWillNotDeleteKeepsTheDomain() {
	service, _, closeAll := test.worldWith(http.StatusCreated, http.StatusForbidden, http.StatusOK)
	defer closeAll()
	added, _ := service.Add(CreateDomainRequest{Name: test.reset("seeded-remove-unit.test")})
	err := service.Remove(added.DomainId)
	stored, getErr := service.Get(added.DomainId)
	var notManaged *RealmNotManaged
	Assert.IsTrue(errors.As(err, &notManaged), "want RealmNotManaged")
	Assert.IsTrue(strings.Contains(err.Error(), added.DomainId), err.Error())
	Assert.IsTrue(getErr == nil, "the domain record is gone")
	Assert.Equals(added.DomainId, stored.DomainId)
}

func (test *DomainServiceTest) TestRemovingAnUnknownDomainIsNotFound() {
	service, _, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	err := service.Remove("unknown-" + test.unique() + ".test")
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *DomainServiceTest) world(createStatus int) (*DomainService, func() string, func()) {
	return test.worldWith(createStatus, http.StatusNoContent, http.StatusNotFound)
}

func (test *DomainServiceTest) worldWith(createStatus int, deleteStatus int, discoveryStatus int) (*DomainService, func() string, func()) {
	statuses := map[string]int{http.MethodPost: createStatus, http.MethodDelete: deleteStatus, http.MethodGet: discoveryStatus}
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
	keycloak := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		test.keycloak(record, statuses, writer, request)
	}))
	dns := httptest.NewServer(http.HandlerFunc(test.dns))
	settings := map[string]any{"yon": map[string]any{"directory": map[string]any{
		"publicurl": "https://mail.example.test", "keycloakurl": keycloak.URL + "/idp", "admin": map[string]any{"client": "mesail-directory", "secret": "example-secret"},
		"realmsecrets": map[string]any{"identity": "example-identity", "service": "example-service", "account": "example-account"},
		"mailhost":     "mx.mesail.test", "spfmechanism": "mx", "dkimselector": "mesail",
		"dnsresolver": dns.URL + "/dns-query",
	}}}
	encoded, _ := json.Marshal(settings)
	os.Setenv("YON_APPLICATION_JSON", string(encoded))
	documents := &DocumentsService{dataSource: DataSource{}}
	settingsService := &SettingsService{}
	service := &DomainService{
		domains:     &DomainStorageService{documents: documents, names: &CollectionNamesService{}},
		collections: &TenantCollectionsService{documents: documents, names: &CollectionNamesService{}},
		realms:      &RealmService{settings: settingsService},
		dns:         &DnsService{settings: settingsService},
		factory:     &DomainFactoryService{settings: settingsService},
	}
	return service, calls, func() { keycloak.Close(); dns.Close() }
}

func (test *DomainServiceTest) keycloak(record func(string), statuses map[string]int, writer http.ResponseWriter, request *http.Request) {
	body, _ := io.ReadAll(request.Body)
	realm := struct {
		Realm string `json:"realm"`
	}{}
	json.Unmarshal(body, &realm)
	record(request.Method + " " + request.URL.Path + " realm=" + realm.Realm)
	if strings.HasSuffix(request.URL.Path, "/token") {
		writer.Write([]byte(`{"access_token":"example-token"}`))
		return
	}
	writer.WriteHeader(statuses[request.Method])
}

func (test *DomainServiceTest) dns(writer http.ResponseWriter, request *http.Request) {
	name := request.URL.Query().Get("name")
	kind := request.URL.Query().Get("type")
	answers := test.records(name, kind)
	response := map[string]any{"Status": 0, "Answer": answers}
	encoded, _ := json.Marshal(response)
	writer.Write(encoded)
}

func (test *DomainServiceTest) records(name string, kind string) []map[string]any {
	trimmed := strings.TrimSuffix(name, ".")
	if kind == "MX" {
		return []map[string]any{{"name": trimmed, "type": 15, "data": "10 mx.mesail.test."}}
	}
	if strings.HasPrefix(trimmed, "_dmarc.partial-") {
		return []map[string]any{}
	}
	text := map[bool]string{true: "\"v=DMARC1; p=reject\"", false: "\"v=spf1 mx -all\""}[strings.HasPrefix(trimmed, "_dmarc.")]
	if strings.HasPrefix(trimmed, "mesail._domainkey.") {
		text = "\"v=DKIM1; k=rsa; p=MIIBIjAN\""
	}
	return []map[string]any{{"name": trimmed, "type": 16, "data": text}}
}

func (test *DomainServiceTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *DomainServiceTest) reset(domainId string) string {
	documents := &DocumentsService{dataSource: DataSource{}}
	names := &CollectionNamesService{}
	domains := &DomainStorageService{documents: documents, names: names}
	old, _, _ := domains.Find(domainId)
	domains.Delete(old)
	(&TenantCollectionsService{documents: documents, names: names}).Drop(domainId)
	return domainId
}
