import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type D2DomainRemovalTest struct {
	behaviors *BehaviorContext
}

func (test *D2DomainRemovalTest) TestRemovingADomainWith200MembersFinishesInBatchesOfAtMost25Writes() {
	removed, kept := test.hosted(), test.hosted()
	defer test.forget(kept)
	people := test.people(removed, kept, 200)
	defer test.forgetPeople(people)
	alias := test.reserveAlias(removed)
	shared := test.reserveAlias(removed, kept)
	defer test.forgetLocalPart(shared)
	_, err := (&DomainService{domains: test.domains()}).Remove(removed)
	reports := test.runUntilDone()
	_, domainFound, _ := test.domains().Find(removed)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(len(reports) >= 9, "want at least 9 batches for 200 people")
	Assert.IsTrue(test.largest(reports) <= 25, "a batch wrote more than 25 times")
	Assert.Equals("0 200", test.stillIn(people, removed, kept))
	Assert.IsTrue(!domainFound, "the removed domain is still stored")
	Assert.Equals(0, test.membersOf(removed))
	Assert.Equals("", test.localPart(alias).LocalPart)
	Assert.Equals(kept, strings.Join(test.localPart(shared).DomainIds, ","))
}

func (test *D2DomainRemovalTest) runUntilDone() []RemovalReport {
	reports := []RemovalReport{}
	for range 60 {
		report, err := test.job().RunOnce()
		Assert.Equals("<nil>", fmt.Sprint(err))
		reports = append(reports, report)
		if !report.More {
			return reports
		}
	}
	return reports
}

func (test *D2DomainRemovalTest) largest(reports []RemovalReport) int {
	largest := 0
	for _, report := range reports {
		largest = max(largest, report.Writes)
	}
	return largest
}

func (test *D2DomainRemovalTest) stillIn(people []PersonDocument, removed string, kept string) string {
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

func (test *D2DomainRemovalTest) membersOf(domainId string) int {
	members, _ := test.memberStorage().All(domainId)
	(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
	return len(members)
}

func (test *D2DomainRemovalTest) people(removed string, kept string, count int) []PersonDocument {
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

func (test *D2DomainRemovalTest) person() PersonDocument {
	return PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Person " + test.unique(), DomainIds: []string{}, AdminDomainIds: []string{}, State: "active"}
}

func (test *D2DomainRemovalTest) stored(person PersonDocument) PersonDocument {
	saved, err := test.peopleStorage().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return saved
}

func (test *D2DomainRemovalTest) reserveAlias(domainIds ...string) string {
	localPart := "alias-" + test.unique()
	for _, domainId := range domainIds {
		Assert.Equals("<nil>", fmt.Sprint(test.reservations().ForAddress(localPart+"@"+domainId)))
	}
	return localPart
}

func (test *D2DomainRemovalTest) localPart(localPart string) LocalPartDocument {
	document, _ := test.localParts().Find(localPart)
	return document
}

func (test *D2DomainRemovalTest) hosted() string {
	domainId := "d2-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	return domainId
}

func (test *D2DomainRemovalTest) forget(domainId string) {
	domain, _, _ := test.domains().Find(domainId)
	test.domains().Delete(domain)
	(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
}

func (test *D2DomainRemovalTest) forgetPeople(people []PersonDocument) {
	for _, person := range people {
		test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
	}
}

func (test *D2DomainRemovalTest) forgetLocalPart(localPart string) {
	test.localParts().Delete(test.localPart(localPart))
}

func (test *D2DomainRemovalTest) job() *RemovalJobService {
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	removal := &DomainRemovalService{domains: test.domains(), people: test.peopleStorage(), localParts: test.localParts(), reservations: test.reservations(), collections: collections}
	store := &OutboxStoreService{domains: test.domains(), members: test.memberStorage(), names: &CollectionNamesService{}}
	relay := &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
	addresses := &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
	writer := &MemberWriterService{members: test.memberStorage(), addresses: addresses, events: &EventService{settings: &SettingsService{}}, relay: relay}
	return &RemovalJobService{domains: removal, people: &PersonLeavingService{people: test.peopleStorage(), writer: writer}}
}

func (test *D2DomainRemovalTest) reservations() *ReservationService {
	return &ReservationService{localParts: test.localParts()}
}

func (test *D2DomainRemovalTest) localParts() *LocalPartStorageService {
	return &LocalPartStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2DomainRemovalTest) peopleStorage() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2DomainRemovalTest) memberStorage() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2DomainRemovalTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *D2DomainRemovalTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *D2DomainRemovalTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
