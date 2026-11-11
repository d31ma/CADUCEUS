import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type L3PeopleSearchTest struct {
	behaviors *BehaviorContext
}

func (test *L3PeopleSearchTest) TestRecordGivesThePersonsDomainsAndDefault() {
	first, second := test.hosted("active"), test.hosted("active")
	person := test.member(first, second)
	defer test.forget([]string{first, second}, person)
	record, err := test.personService().Record(person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(person.Username, record.Username)
	Assert.Equals(first+","+second, strings.Join(record.DomainIds, ","))
	Assert.Equals(first, record.DefaultDomainId)
	Assert.Equals("active", record.State)
}

func (test *L3PeopleSearchTest) TestRecordOfAnUnknownPersonIsNotFound() {
	_, err := test.personService().Record("nobody-" + test.unique())
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3PeopleSearchTest) TestSearchFindsPeopleByUsernameAndDisplayName() {
	tag := test.unique()
	byUsername := test.stored(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + tag, DisplayName: "First", State: "active"})
	byName := test.stored(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Name " + strings.ToUpper(tag), State: "active"})
	other := test.stored(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Other", State: "active"})
	defer test.forget([]string{}, byUsername, byName, other)
	page, err := test.personService().Search(MemberQuery{Text: " " + tag + " "})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(2, len(page.Items))
	Assert.Equals(byUsername.Username, test.personItem(page, byUsername.PersonId).Username)
	Assert.Equals(byName.DisplayName, test.personItem(page, byName.PersonId).DisplayName)
	Assert.Equals("", page.NextCursor)
}

func (test *L3PeopleSearchTest) TestSearchPagesWithACursor() {
	tag := test.unique()
	one := test.stored(PersonDocument{PersonId: "p-" + test.unique(), Username: "a" + tag, State: "active"})
	two := test.stored(PersonDocument{PersonId: "p-" + test.unique(), Username: "b" + tag, State: "active"})
	three := test.stored(PersonDocument{PersonId: "p-" + test.unique(), Username: "c" + tag, State: "active"})
	defer test.forget([]string{}, one, two, three)
	first, firstErr := test.personService().Search(MemberQuery{Text: tag, Limit: 2})
	second, secondErr := test.personService().Search(MemberQuery{Text: tag, Limit: 2, Cursor: first.NextCursor})
	Assert.Equals("<nil>", fmt.Sprint(firstErr))
	Assert.Equals("<nil>", fmt.Sprint(secondErr))
	Assert.Equals(2, len(first.Items))
	Assert.Equals("2", first.NextCursor)
	Assert.Equals(1, len(second.Items))
	Assert.Equals("", second.NextCursor)
}

func (test *L3PeopleSearchTest) TestSearchRefusesALimitAboveOneHundred() {
	_, err := test.personService().Search(MemberQuery{Limit: 101})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("limit", invalid.Field)
}

func (test *L3PeopleSearchTest) TestSearchRefusesACursorItDidNotGiveOut() {
	_, err := test.personService().Search(MemberQuery{Cursor: "abc"})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("cursor", invalid.Field)
}

func (test *L3PeopleSearchTest) TestAnAdminSeesUsernamesAdminFlagsAndManagedAccounts() {
	domainId, otherDomainId := test.hosted("active"), test.hosted("active")
	admin := test.admin(domainId)
	local := test.member(domainId)
	shared := test.member(domainId, otherDomainId)
	defer test.forget([]string{domainId, otherDomainId}, admin, local, shared)
	page, err := test.memberService().Search(Admission{Caller: test.domainAdmin(admin, domainId)}, MemberQuery{}, nil)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(3, len(page.Items))
	Assert.Equals(local.Username, test.memberItem(page, local.PersonId).Username)
	Assert.IsTrue(test.memberItem(page, local.PersonId).AccountManaged, "the admin manages a person of only this domain")
	Assert.IsTrue(!test.memberItem(page, shared.PersonId).AccountManaged, "the admin does not manage a person of another domain too")
	Assert.IsTrue(test.memberItem(page, admin.PersonId).Admin, "want the admin flag")
	Assert.IsTrue(!test.memberItem(page, local.PersonId).Admin, "a plain member is not an admin")
}

func (test *L3PeopleSearchTest) TestAnOperatorManagesEveryAccount() {
	domainId, otherDomainId := test.hosted("active"), test.hosted("active")
	shared := test.member(domainId, otherDomainId)
	defer test.forget([]string{domainId, otherDomainId}, shared)
	page, err := test.memberService().Search(Admission{Caller: oidc.Caller{DomainId: domainId, MemberId: "operator-" + test.unique(), Roles: []string{"Operator"}}}, MemberQuery{}, nil)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(shared.Username, test.memberItem(page, shared.PersonId).Username)
	Assert.IsTrue(test.memberItem(page, shared.PersonId).AccountManaged, "an operator manages every account")
}

func (test *L3PeopleSearchTest) TestAPlainMemberSeesNoUsernames() {
	domainId := test.hosted("active")
	person := test.member(domainId)
	viewer := test.member(domainId)
	defer test.forget([]string{domainId}, person, viewer)
	page, err := test.memberService().Search(Admission{Caller: oidc.Caller{DomainId: domainId, MemberId: viewer.PersonId, Roles: []string{"Member"}}}, MemberQuery{}, nil)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(2, len(page.Items))
	Assert.Equals("", test.memberItem(page, person.PersonId).Username)
	Assert.Equals(person.DisplayName, test.memberItem(page, person.PersonId).DisplayName)
}

func (test *L3PeopleSearchTest) TestRemovedMembersAreNotListed() {
	domainId := test.hosted("active")
	person := test.member(domainId)
	removed := test.member(domainId)
	defer test.forget([]string{domainId}, person, removed)
	Assert.Equals("<nil>", fmt.Sprint(test.writer().LeaveDomain(removed.PersonId, domainId)))
	page, err := test.memberService().Search(Admission{Caller: oidc.Caller{DomainId: domainId, MemberId: person.PersonId, Roles: []string{"DomainAdmin"}}}, MemberQuery{}, nil)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(1, len(page.Items))
	Assert.Equals(person.PersonId, page.Items[0].MemberId)
}

func (test *L3PeopleSearchTest) personService() *PersonService {
	return &PersonService{people: test.people(), access: test.access(), accounts: test.accounts(), writer: test.writer(), domains: test.domains()}
}

func (test *L3PeopleSearchTest) memberService() *MemberService {
	return &MemberService{members: test.memberStorage(), people: test.people(), access: test.access()}
}

func (test *L3PeopleSearchTest) access() *PersonAccessService {
	return &PersonAccessService{people: test.people(), accounts: test.accounts()}
}

func (test *L3PeopleSearchTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *L3PeopleSearchTest) writer() *MemberWriterService {
	return &MemberWriterService{members: test.memberStorage(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: test.relay()}
}

func (test *L3PeopleSearchTest) relay() *OutboxRelayService {
	store := &OutboxStoreService{domains: test.domains(), members: test.memberStorage(), names: &CollectionNamesService{}}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
}

func (test *L3PeopleSearchTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleSearchTest) memberStorage() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleSearchTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleSearchTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3PeopleSearchTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3PeopleSearchTest) hosted(state string) string {
	domainId := "l3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: state})))
	return domainId
}

func (test *L3PeopleSearchTest) member(domainIds ...string) PersonDocument {
	return test.joined(PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Person " + test.unique(), State: "active"}, false, domainIds)
}

func (test *L3PeopleSearchTest) admin(domainIds ...string) PersonDocument {
	return test.joined(PersonDocument{PersonId: "a-" + test.unique(), Username: "a" + test.unique(), DisplayName: "Admin " + test.unique(), State: "active"}, true, domainIds)
}

func (test *L3PeopleSearchTest) joined(person PersonDocument, admin bool, domainIds []string) PersonDocument {
	for _, domainId := range domainIds {
		person = PersonRules{}.WithAdmin(PersonRules{}.With(person, domainId), domainId, admin)
		_, err := test.writer().Join(person, domainId, admin)
		Assert.Equals("<nil>", fmt.Sprint(err))
	}
	return test.stored(person)
}

func (test *L3PeopleSearchTest) stored(person PersonDocument) PersonDocument {
	saved, err := test.people().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return saved
}

func (test *L3PeopleSearchTest) forget(domainIds []string, people ...PersonDocument) {
	for _, person := range people {
		test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
	}
	for _, domainId := range domainIds {
		domain, _, _ := test.domains().Find(domainId)
		test.domains().Delete(domain)
		(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
	}
}

func (test *L3PeopleSearchTest) personItem(page PersonPage, personId string) PersonSummary {
	for _, item := range page.Items {
		if item.PersonId == personId {
			return item
		}
	}
	return PersonSummary{}
}

func (test *L3PeopleSearchTest) memberItem(page MemberPage, memberId string) MemberSummary {
	for _, item := range page.Items {
		if item.MemberId == memberId {
			return item
		}
	}
	return MemberSummary{}
}

func (test *L3PeopleSearchTest) domainAdmin(admin PersonDocument, domainId string) oidc.Caller {
	return oidc.Caller{DomainId: domainId, MemberId: admin.PersonId, Username: admin.Username, Roles: []string{"DomainAdmin"}}
}

func (test *L3PeopleSearchTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
