import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand/v2"
	"time"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type K6DomainTurnsTest struct {
	behaviors *BehaviorContext
}

func (test *K6DomainTurnsTest) TestASmallDomainIsRemovedWhileABigOneIsStillRemoving() {
	big, small, kept := test.hosted(), test.hosted(), test.hosted()
	defer test.forget(kept)
	people := append(test.people(big, kept, 40), test.people(small, kept, 3)...)
	defer test.forgetPeople(people)
	_, bigErr := (&DomainService{domains: test.domains()}).Remove(big)
	_, smallErr := (&DomainService{domains: test.domains()}).Remove(small)
	reports := test.runUntilGone(small, 12)
	_, smallFound, _ := test.domains().Find(small)
	_, bigFound, _ := test.domains().Find(big)
	finished := test.runUntilDone()
	Assert.Equals("<nil>", fmt.Sprint(bigErr))
	Assert.Equals("<nil>", fmt.Sprint(smallErr))
	Assert.IsTrue(!smallFound, "the small domain waited behind the big one")
	Assert.IsTrue(bigFound, "want the big domain still removing")
	Assert.IsTrue(test.largest(append(reports, finished...)) <= 3, "a batch wrote more than 3 times")
	Assert.Equals("0 43", test.stillIn(people, big, kept))
}

func (test *K6DomainTurnsTest) TestADomainGoingFirstFinishesWhileAPersonIsStillLeaving() {
	busy, kept := test.hosted(), test.hosted()
	defer test.forget(busy)
	defer test.forget(kept)
	leavers := test.leaving(test.people(busy, kept, 3))
	defer test.forgetPeople(leavers)
	early, late := test.hostedFirst(), test.hostedFirst()
	defer test.forget(late)
	_, earlyErr := (&DomainService{domains: test.domains()}).Remove(early)
	domainsFirst, domainsErr := test.job().runFrom(0, false)
	_, earlyFound, _ := test.domains().Find(early)
	afterDomains, _, _ := test.peopleStorage().ByPersonId(leavers[0].PersonId)
	_, lateErr := (&DomainService{domains: test.domains()}).Remove(late)
	peopleFirst, peopleErr := test.job().runFrom(0, true)
	_, lateFound, _ := test.domains().Find(late)
	Assert.Equals("<nil>", fmt.Sprint(earlyErr))
	Assert.Equals("<nil>", fmt.Sprint(lateErr))
	Assert.Equals("<nil>", fmt.Sprint(domainsErr))
	Assert.Equals("<nil>", fmt.Sprint(peopleErr))
	Assert.IsTrue(!earlyFound, "want the domain removed in a call that starts with domains")
	Assert.Equals(2, len(afterDomains.DomainIds))
	Assert.Equals(RemovalReport{Writes: 3, More: true}, domainsFirst)
	Assert.IsTrue(lateFound, "want the domain to wait in a call that starts with people")
	Assert.Equals(RemovalReport{Writes: 3, More: true}, peopleFirst)
}

func (test *K6DomainTurnsTest) TestADomainDropTakesOneWritePerPartAndSkipsTheDroppedParts() {
	domainId := test.hosted()
	_, err := (&DomainService{domains: test.domains()}).Remove(domainId)
	first := test.finishWithOneWrite(domainId)
	membersLeft, _ := test.documents().Exists((&CollectionNamesService{}).Members(domainId))
	addressesAfterFirst, _ := test.documents().Exists((&CollectionNamesService{}).Addresses(domainId))
	second := test.finishWithOneWrite(domainId)
	_, foundAfterSecond, _ := test.domains().Find(domainId)
	third := test.finishWithOneWrite(domainId)
	_, foundAfterThird, _ := test.domains().Find(domainId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("1 1 1", fmt.Sprint(first, second, third))
	Assert.IsTrue(!membersLeft, "want the members collection dropped by the first call")
	Assert.IsTrue(addressesAfterFirst, "want the addresses collection kept for the next call")
	Assert.IsTrue(foundAfterSecond, "want the domain document kept until both collections are gone")
	Assert.IsTrue(!foundAfterThird, "want the domain document deleted by the third call")
}

func (test *K6DomainTurnsTest) finishWithOneWrite(domainId string) int {
	domain, _, _ := test.domains().Find(domainId)
	budget := &WriteBudget{Left: 1, Deadline: time.Now().Add(time.Minute)}
	Assert.Equals("<nil>", fmt.Sprint(test.removal().finish(domain, budget)))
	return budget.Report().Writes
}

func (test *K6DomainTurnsTest) leaving(people []PersonDocument) []PersonDocument {
	left := []PersonDocument{}
	for _, person := range people {
		person.State = "deleted"
		left = append(left, test.stored(person))
	}
	return left
}

func (test *K6DomainTurnsTest) hostedFirst() string {
	domainId := "0k6-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	return domainId
}

func (test *K6DomainTurnsTest) runUntilGone(domainId string, calls int) []RemovalReport {
	random := mathrand.New(mathrand.NewPCG(6, 6))
	reports := []RemovalReport{}
	for range calls {
		report, err := test.job().runFrom(random.Int(), random.IntN(2) == 0)
		Assert.Equals("<nil>", fmt.Sprint(err))
		reports = append(reports, report)
		if _, found, _ := test.domains().Find(domainId); !found {
			return reports
		}
	}
	return reports
}

func (test *K6DomainTurnsTest) runUntilDone() []RemovalReport {
	reports := []RemovalReport{}
	for range 150 {
		report, err := test.job().RunOnce()
		Assert.Equals("<nil>", fmt.Sprint(err))
		reports = append(reports, report)
		if !report.More {
			return reports
		}
	}
	return reports
}

func (test *K6DomainTurnsTest) largest(reports []RemovalReport) int {
	largest := 0
	for _, report := range reports {
		largest = max(largest, report.Writes)
	}
	return largest
}

func (test *K6DomainTurnsTest) stillIn(people []PersonDocument, removed string, kept string) string {
	wanted := map[string]bool{}
	for _, person := range people {
		wanted[person.PersonId] = true
	}
	stored, err := test.peopleStorage().All()
	Assert.Equals("<nil>", fmt.Sprint(err))
	counts := map[string]int{}
	for _, person := range stored {
		for _, domainId := range person.DomainIds {
			counts[domainId] += map[bool]int{true: 1}[wanted[person.PersonId]]
		}
	}
	return fmt.Sprint(counts[removed], " ", counts[kept])
}

func (test *K6DomainTurnsTest) people(removed string, kept string, count int) []PersonDocument {
	people := []PersonDocument{}
	for index := range count {
		person := PersonRules{}.WithAdmin(PersonRules{}.With(PersonRules{}.With(test.person(), removed), kept), removed, index == 0)
		people = append(people, test.stored(person))
	}
	for _, person := range people[:3] {
		_, err := test.memberStorage().Insert(MemberDocument{MemberId: person.PersonId, DomainId: removed, Username: person.Username, State: "active"})
		Assert.Equals("<nil>", fmt.Sprint(err))
	}
	return people
}

func (test *K6DomainTurnsTest) person() PersonDocument {
	return PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Person " + test.unique(), DomainIds: []string{}, AdminDomainIds: []string{}, State: "active"}
}

func (test *K6DomainTurnsTest) stored(person PersonDocument) PersonDocument {
	saved, err := test.peopleStorage().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return saved
}

func (test *K6DomainTurnsTest) hosted() string {
	domainId := "d2-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	return domainId
}

func (test *K6DomainTurnsTest) forget(domainId string) {
	domain, _, _ := test.domains().Find(domainId)
	test.domains().Delete(domain)
	(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId, &WriteBudget{Left: 2, Deadline: time.Now().Add(time.Minute)})
}

func (test *K6DomainTurnsTest) forgetPeople(people []PersonDocument) {
	for _, person := range people {
		test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
	}
}

func (test *K6DomainTurnsTest) removal() *DomainRemovalService {
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	return &DomainRemovalService{domains: test.domains(), people: test.peopleStorage(), localParts: test.localParts(), reservations: test.reservations(), collections: collections}
}

func (test *K6DomainTurnsTest) job() *RemovalJobService {
	removal := test.removal()
	store := &OutboxStoreService{domains: test.domains(), members: test.memberStorage(), names: &CollectionNamesService{}}
	relay := &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
	addresses := &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
	writer := &MemberWriterService{members: test.memberStorage(), addresses: addresses, events: &EventService{settings: &SettingsService{}}, relay: relay}
	return &RemovalJobService{domains: removal, people: &PersonLeavingService{people: test.peopleStorage(), writer: writer}}
}

func (test *K6DomainTurnsTest) reservations() *ReservationService {
	return &ReservationService{localParts: test.localParts()}
}

func (test *K6DomainTurnsTest) localParts() *LocalPartStorageService {
	return &LocalPartStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *K6DomainTurnsTest) peopleStorage() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *K6DomainTurnsTest) memberStorage() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *K6DomainTurnsTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *K6DomainTurnsTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *K6DomainTurnsTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
