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
	name := "Add-" + test.unique() + ".TEST"
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
	name := "twice-" + test.unique() + ".test"
	service.Add(CreateDomainRequest{Name: name})
	_, err := service.Add(CreateDomainRequest{Name: name})
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *DomainServiceTest) TestARealmThatAlreadyExistsIsAdopted() {
	service, _, closeAll := test.world(http.StatusConflict)
	defer closeAll()
	_, err := service.Add(CreateDomainRequest{Name: "adopt-" + test.unique() + ".test"})
	Assert.IsTrue(err == nil, "add failed")
}

func (test *DomainServiceTest) TestAKeycloakFailureLeavesNoDomain() {
	service, _, closeAll := test.world(http.StatusInternalServerError)
	defer closeAll()
	name := "fail-" + test.unique() + ".test"
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
	added, _ := service.Add(CreateDomainRequest{Name: "good-" + test.unique() + ".test"})
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
	added, _ := service.Add(CreateDomainRequest{Name: "partial-" + test.unique() + ".test"})
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
	added, _ := service.Add(CreateDomainRequest{Name: "list-" + test.unique() + ".test"})
	domains, err := service.List()
	encoded, _ := json.Marshal(domains)
	Assert.IsTrue(err == nil, "list failed")
	Assert.IsTrue(strings.Contains(string(encoded), `"domainId":"`+added.DomainId+`"`), "domain not listed")
}

func (test *DomainServiceTest) TestRemovingADomainDeletesItsRealmAndRecord() {
	service, calls, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	added, _ := service.Add(CreateDomainRequest{Name: "remove-" + test.unique() + ".test"})
	err := service.Remove(added.DomainId)
	_, getErr := service.Get(added.DomainId)
	var missing *NotFound
	Assert.IsTrue(err == nil, "remove failed")
	Assert.IsTrue(errors.As(getErr, &missing), "want NotFound")
	Assert.IsTrue(strings.Contains(calls(), "DELETE /idp/admin/realms/"+added.DomainId), calls())
}

func (test *DomainServiceTest) TestRemovingAnUnknownDomainIsNotFound() {
	service, _, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	err := service.Remove("unknown-" + test.unique() + ".test")
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *DomainServiceTest) world(createStatus int) (*DomainService, func() string, func()) {
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
		test.keycloak(record, createStatus, writer, request)
	}))
	dns := httptest.NewServer(http.HandlerFunc(test.dns))
	settings := map[string]any{"yon": map[string]any{"directory": map[string]any{
		"publicurl": "https://mail.example.test", "keycloakurl": keycloak.URL + "/idp", "admin": map[string]any{"client": "mesail-directory", "secret": "example-secret"},
		"mailhost": "mx.mesail.test", "spfmechanism": "mx", "dkimselector": "mesail",
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

func (test *DomainServiceTest) keycloak(record func(string), createStatus int, writer http.ResponseWriter, request *http.Request) {
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
	if request.Method == http.MethodPost {
		writer.WriteHeader(createStatus)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
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
