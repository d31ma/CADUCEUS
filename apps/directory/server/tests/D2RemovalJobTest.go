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
	"time"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type D2RemovalJobTest struct {
	behaviors *BehaviorContext
}

func (test *D2RemovalJobTest) TestARemovingDomainLeavesThePersonRecordAtOnce() {
	removed, kept := test.hosted(), test.hosted()
	defer test.forget(removed, kept)
	person := test.stored(PersonRules{}.WithAdmin(PersonRules{}.With(PersonRules{}.With(test.person(), removed), kept), removed, true))
	defer test.forgetPeople([]PersonDocument{person})
	_, err := test.domainService().Remove(removed)
	record, recordErr := test.personService().Record(person.PersonId)
	stored, _, _ := test.peopleStorage().ByPersonId(person.PersonId)
	_, refused := (&TenantService{domains: test.domains()}).Require(removed)
	var forbidden *Forbidden
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint(recordErr))
	Assert.Equals(kept, strings.Join(record.DomainIds, ","))
	Assert.Equals(0, len(record.AdminDomainIds))
	Assert.Equals(kept, record.DefaultDomainId)
	Assert.Equals(removed+","+kept, strings.Join(stored.DomainIds, ","))
	Assert.IsTrue(errors.As(refused, &forbidden), "want the removing domain refused")
}

func (test *D2RemovalJobTest) TestDeletingAPersonInFiveDomainsRemovesEveryMembership() {
	base, calls, closeServer := test.keycloak()
	defer closeServer()
	test.configure(base)
	domains := []string{test.hosted(), test.hosted(), test.hosted(), test.hosted(), test.hosted()}
	defer test.forget(domains...)
	person := test.joined(test.person(), domains)
	defer test.forgetPeople([]PersonDocument{person})
	err := test.personService().Delete(person.PersonId)
	marked, _, _ := test.peopleStorage().ByPersonId(person.PersonId)
	reports := test.runUntilDone()
	stored, _, _ := test.peopleStorage().ByPersonId(person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(strings.Contains(calls(), "DELETE /idp/admin/realms/mesail/users/"+person.PersonId), "want the account deleted")
	Assert.Equals("deleted", marked.State)
	Assert.Equals(5, len(marked.DomainIds))
	Assert.IsTrue(test.largest(reports) <= 25, "a batch wrote more than 25 times")
	Assert.Equals("removed,removed,removed,removed,removed", test.memberStates(domains, person.PersonId))
	Assert.Equals(0, len(stored.DomainIds))
	Assert.Equals("deleted", stored.State)
}

func (test *D2RemovalJobTest) TestPersonLeavingStopsWhenTheBudgetRunsOutAndContinuesLater() {
	domains := []string{test.hosted(), test.hosted(), test.hosted()}
	defer test.forget(domains...)
	person := test.joined(test.person(), domains)
	defer test.forgetPeople([]PersonDocument{person})
	person.State = "deleted"
	person = test.stored(person)
	budget := &WriteBudget{Left: 8, Deadline: time.Now().Add(time.Minute)}
	err := test.leaving().leaveEachDomain(person, budget)
	partial, _, _ := test.peopleStorage().ByPersonId(person.PersonId)
	report := budget.Report()
	finishErr := test.leaving().leaveEachDomain(partial, &WriteBudget{Left: 25, Deadline: time.Now().Add(time.Minute)})
	stored, _, _ := test.peopleStorage().ByPersonId(person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint(finishErr))
	Assert.Equals(8, report.Writes)
	Assert.IsTrue(report.More, "want more work reported")
	Assert.Equals(domains[2], strings.Join(partial.DomainIds, ","))
	Assert.Equals(0, len(stored.DomainIds))
}

func (test *D2RemovalJobTest) TestABudgetPastItsDeadlineTakesNothing() {
	budget := &WriteBudget{Left: 25, Deadline: time.Now().Add(-time.Second)}
	Assert.IsTrue(!budget.Take(1), "a budget past its deadline must refuse")
	Assert.Equals(RemovalReport{Writes: 0, More: true}, budget.Report())
}

func (test *D2RemovalJobTest) TestReleaseDomainKeepsTheOtherDomainsOfAnAlias() {
	first, second := test.hosted(), test.hosted()
	defer test.forget(first, second)
	localPart := test.reserveAlias(first, second)
	defer test.forgetLocalPart(localPart)
	err := test.reservations().ReleaseDomain(test.localPart(localPart), first)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(second, strings.Join(test.localPart(localPart).DomainIds, ","))
}

func (test *D2RemovalJobTest) TestReleaseDomainDeletesAnAliasReservationWithoutDomains() {
	domain := test.hosted()
	defer test.forget(domain)
	localPart := test.reserveAlias(domain)
	err := test.reservations().ReleaseDomain(test.localPart(localPart), domain)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(!test.localPartFound(localPart), "want the empty reservation deleted")
}

func (test *D2RemovalJobTest) TestReleaseDomainNeverReleasesAPersonsPrefix() {
	domain := test.hosted()
	defer test.forget(domain)
	username := "u" + test.unique()
	Assert.Equals("<nil>", fmt.Sprint(test.reservations().ForPerson(username, "p-"+test.unique())))
	defer test.forgetLocalPart(username)
	err := test.reservations().ReleaseDomain(test.localPart(username), domain)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("person", test.localPart(username).Owner)
}

func (test *D2RemovalJobTest) TestTheRemovalsRouteRefusesARequestWithoutAValidToken() {
	controller := &JobsController{callers: &CallerService{}}
	_, err := controller.PostRemovals("Bearer forged")
	var refused *Unauthenticated
	Assert.IsTrue(errors.As(err, &refused), "want Unauthenticated")
}

func (test *D2RemovalJobTest) TestTheRemovalsRouteRunsTheJobForAnInstallWideServiceToken() {
	setup := DirectoryTestSetup{}
	realm := setup.SignedRealm()
	base, _, closeServer := test.keycloak()
	defer closeServer()
	os.Setenv("YON_APPLICATION_JSON", setup.ApplicationJson(realm, base))
	controller := &JobsController{callers: &CallerService{}, removals: test.job()}
	_, err := controller.PostRemovals(setup.ServiceBearer(realm, "*"))
	Assert.Equals("<nil>", fmt.Sprint(err))
}

func (test *D2RemovalJobTest) TestTheRemovalsRouteHidesItselfFromAMailDomainServiceToken() {
	setup := DirectoryTestSetup{}
	realm := setup.SignedRealm()
	os.Setenv("YON_APPLICATION_JSON", setup.ApplicationJson(realm, "http://127.0.0.1:1"))
	controller := &JobsController{callers: &CallerService{}, removals: test.job()}
	_, err := controller.PostRemovals(setup.ServiceBearer(realm, "example.com"))
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *D2RemovalJobTest) TestBatchUtilsRunsEveryStepAndKeepsEveryFailure() {
	ran := []int{}
	err := BatchUtils{}.Each(3, func(index int) error {
		ran = append(ran, index)
		return fmt.Errorf("step %d failed", index)
	})
	Assert.Equals("[0 1 2]", fmt.Sprint(ran))
	Assert.Equals("step 0 failed\nstep 1 failed\nstep 2 failed", err.Error())
}

func (test *D2RemovalJobTest) runUntilDone() []RemovalReport {
	reports := []RemovalReport{}
	for range 40 {
		report, err := test.job().RunOnce()
		Assert.Equals("<nil>", fmt.Sprint(err))
		reports = append(reports, report)
		if !report.More {
			return reports
		}
	}
	return reports
}

func (test *D2RemovalJobTest) largest(reports []RemovalReport) int {
	largest := 0
	for _, report := range reports {
		largest = max(largest, report.Writes)
	}
	return largest
}

func (test *D2RemovalJobTest) memberStates(domainIds []string, personId string) string {
	states := []string{}
	for _, domainId := range domainIds {
		member, _, _ := test.memberStorage().Find(domainId, personId)
		states = append(states, member.State)
	}
	return strings.Join(states, ",")
}

func (test *D2RemovalJobTest) localPart(localPart string) LocalPartDocument {
	document, _ := test.localParts().Find(localPart)
	return document
}

func (test *D2RemovalJobTest) localPartFound(localPart string) bool {
	return test.localPart(localPart).LocalPart != ""
}

func (test *D2RemovalJobTest) reserveAlias(domainIds ...string) string {
	localPart := "alias-" + test.unique()
	for _, domainId := range domainIds {
		Assert.Equals("<nil>", fmt.Sprint(test.reservations().ForAddress(localPart+"@"+domainId)))
	}
	return localPart
}

func (test *D2RemovalJobTest) person() PersonDocument {
	return PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Person " + test.unique(), DomainIds: []string{}, AdminDomainIds: []string{}, State: "active"}
}

func (test *D2RemovalJobTest) joined(person PersonDocument, domainIds []string) PersonDocument {
	for _, domainId := range domainIds {
		person = PersonRules{}.With(person, domainId)
		_, err := test.writer().Join(person, domainId, false)
		Assert.Equals("<nil>", fmt.Sprint(err))
	}
	return test.stored(person)
}

func (test *D2RemovalJobTest) stored(person PersonDocument) PersonDocument {
	saved, err := test.peopleStorage().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return saved
}

func (test *D2RemovalJobTest) hosted() string {
	domainId := "d2-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	return domainId
}

func (test *D2RemovalJobTest) forget(domainIds ...string) {
	for _, domainId := range domainIds {
		domain, found, _ := test.domains().Find(domainId)
		if found {
			test.domains().Delete(domain)
		}
		(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
	}
}

func (test *D2RemovalJobTest) forgetPeople(people []PersonDocument) {
	for _, person := range people {
		test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
	}
}

func (test *D2RemovalJobTest) forgetLocalPart(localPart string) {
	document := test.localPart(localPart)
	if document.Id != "" {
		test.localParts().Delete(document)
	}
}

func (test *D2RemovalJobTest) job() *RemovalJobService {
	return &RemovalJobService{domains: test.removal(), people: test.leaving()}
}

func (test *D2RemovalJobTest) removal() *DomainRemovalService {
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	return &DomainRemovalService{domains: test.domains(), people: test.peopleStorage(), localParts: test.localParts(), reservations: test.reservations(), collections: collections}
}

func (test *D2RemovalJobTest) leaving() *PersonLeavingService {
	return &PersonLeavingService{people: test.peopleStorage(), writer: test.writer()}
}

func (test *D2RemovalJobTest) domainService() *DomainService {
	return &DomainService{domains: test.domains()}
}

func (test *D2RemovalJobTest) personService() *PersonService {
	accounts := &AccountAdminService{settings: &SettingsService{}}
	access := &PersonAccessService{people: test.peopleStorage(), accounts: accounts}
	return &PersonService{people: test.peopleStorage(), access: access, accounts: accounts, writer: test.writer(), domains: test.domains()}
}

func (test *D2RemovalJobTest) writer() *MemberWriterService {
	store := &OutboxStoreService{domains: test.domains(), members: test.memberStorage(), names: &CollectionNamesService{}}
	relay := &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
	return &MemberWriterService{members: test.memberStorage(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: relay}
}

func (test *D2RemovalJobTest) reservations() *ReservationService {
	return &ReservationService{localParts: test.localParts()}
}

func (test *D2RemovalJobTest) localParts() *LocalPartStorageService {
	return &LocalPartStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2RemovalJobTest) peopleStorage() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2RemovalJobTest) memberStorage() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2RemovalJobTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2RemovalJobTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2RemovalJobTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *D2RemovalJobTest) configure(base string) {
	settings := `{"yon":{"directory":{"keycloakurl":%q,"admin":{"client":"mesail-directory","secret":"example-secret"}}}}`
	os.Setenv("YON_APPLICATION_JSON", fmt.Sprintf(settings, base+"/idp"))
}

func (test *D2RemovalJobTest) keycloak() (string, func() string, func()) {
	lock := &sync.Mutex{}
	lines := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/protocol/openid-connect/token") {
			writer.Write([]byte(`{"access_token":"example-token"}`))
			return
		}
		lock.Lock()
		lines = append(lines, request.Method+" "+request.URL.Path)
		lock.Unlock()
		writer.WriteHeader(http.StatusNoContent)
	}))
	calls := func() string {
		lock.Lock()
		defer lock.Unlock()
		return strings.Join(lines, "\n")
	}
	return server.URL, calls, server.Close
}

func (test *D2RemovalJobTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
