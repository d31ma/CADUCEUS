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
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type DomainServiceTest struct {
	behaviors *BehaviorContext
}

func (test *DomainServiceTest) TestAddingADomainStoresItPendingWithoutCallingKeycloak() {
	service, calls, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	name := "Add-" + test.reset("add-unit.test")[4:]
	domain, err := service.Add(CreateDomainRequest{Name: name})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(strings.ToLower(name), domain.DomainId)
	Assert.Equals("pending", domain.State)
	Assert.Equals("mesail", domain.DkimSelector)
	Assert.Equals("missing", domain.Dns.Mx)
	Assert.Equals("", calls())
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
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("active", domain.State)
	Assert.Equals("ok", domain.Dns.Dkim)
	Assert.Equals("active", stored.State)
}

func (test *DomainServiceTest) TestVerifyWithAMissingRecordKeepsTheDomainPending() {
	service, _, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	added, _ := service.Add(CreateDomainRequest{Name: test.reset("partial-unit.test")})
	domain, err := service.Verify(added.DomainId)
	Assert.Equals("<nil>", fmt.Sprint(err))
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
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(strings.Contains(string(encoded), `"domainId":"`+added.DomainId+`"`), "domain not listed")
}

func (test *DomainServiceTest) TestRemovingADomainMarksItRemovingWithoutCallingKeycloak() {
	service, calls, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	added, _ := service.Add(CreateDomainRequest{Name: test.reset("remove-unit.test")})
	removed, err := service.Remove(added.DomainId)
	stored, getErr := service.Get(added.DomainId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint(getErr))
	Assert.Equals("removing", removed.State)
	Assert.Equals("removing", stored.State)
	Assert.Equals("", calls())
}

func (test *DomainServiceTest) TestRemovingADomainTwiceKeepsItRemoving() {
	service, _, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	added, _ := service.Add(CreateDomainRequest{Name: test.reset("remove-twice-unit.test")})
	service.Remove(added.DomainId)
	removed, err := service.Remove(added.DomainId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("removing", removed.State)
}

func (test *DomainServiceTest) TestRemovingAnUnknownDomainIsNotFound() {
	service, _, closeAll := test.world(http.StatusCreated)
	defer closeAll()
	_, err := service.Remove("unknown-" + test.unique() + ".test")
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
