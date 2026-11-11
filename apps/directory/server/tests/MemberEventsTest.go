import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type MemberEventsTest struct {
	behaviors *BehaviorContext
}

func (test *MemberEventsTest) TestRememberPublishesTheMemberEvent() {
	service, storage, domain := test.world()
	service.Remember(test.caller(domain, "Bo Member"))
	stored, _, _ := storage.Find(domain, "bo-id")
	Assert.Equals(1, len(stored.PendingEvents))
	Assert.IsTrue(strings.Contains(stored.PendingEvents[0], `"type":"member.upserted"`), "wrong event type")
	Assert.IsTrue(strings.Contains(stored.PendingEvents[0], `"domainId":"`+domain+`"`), "wrong event domain")
}

func (test *MemberEventsTest) TestRememberingTheSameTokenTwicePublishesOnce() {
	service, storage, domain := test.world()
	service.Remember(test.caller(domain, "Bo Member"))
	service.Remember(test.caller(domain, "Bo Member"))
	stored, _, _ := storage.Find(domain, "bo-id")
	Assert.Equals(1, len(stored.PendingEvents))
}

func (test *MemberEventsTest) TestANewNameIsSavedAndPublished() {
	service, storage, domain := test.world()
	service.Remember(test.caller(domain, "Bo Member"))
	service.Remember(test.caller(domain, "Bo Renamed"))
	stored, _, _ := storage.Find(domain, "bo-id")
	Assert.Equals("Bo Renamed", stored.DisplayName)
	Assert.Equals(2, len(stored.PendingEvents))
}

func (test *MemberEventsTest) caller(domain string, name string) oidc.Caller {
	return oidc.Caller{DomainId: domain, MemberId: "bo-id", DisplayName: name, Email: "Bo@" + strings.ToUpper(domain)}
}

func (test *MemberEventsTest) world() (*MemberService, *MemberStorageService, string) {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{"outbox":{"producer":"directory","subscribers":[{"name":"search","url":"http://127.0.0.1:9/api/v1/search/events"}]}}}`)
	documents := &DocumentsService{dataSource: DataSource{}}
	members := &MemberStorageService{documents: documents, names: &CollectionNamesService{}}
	addresses := &AddressStorageService{documents: documents, names: &CollectionNamesService{}}
	tenants := &TenantService{domains: &DomainStorageService{documents: documents, names: &CollectionNamesService{}}}
	return &MemberService{tenants: tenants, members: members, addresses: addresses, events: &EventService{settings: &SettingsService{}}, relay: test.relay(documents)}, members, test.hosted("events-unit.test", "active")
}

func (test *MemberEventsTest) hosted(name string, state string) string {
	domainId := test.reset(name)
	documents := &DocumentsService{dataSource: DataSource{}}
	collections := &TenantCollectionsService{documents: documents, names: &CollectionNamesService{}}
	Assert.IsTrue(collections.Prepare(domainId) == nil, "prepare failed")
	Assert.IsTrue((&DomainStorageService{documents: documents, names: &CollectionNamesService{}}).Insert(DomainDocument{DomainId: domainId, Name: domainId, State: state}) == nil, "insert failed")
	return domainId
}

func (test *MemberEventsTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *MemberEventsTest) relay(documents *DocumentsService) *OutboxRelayService {
	names := &CollectionNamesService{}
	store := &OutboxStoreService{domains: &DomainStorageService{documents: documents, names: names}, members: &MemberStorageService{documents: documents, names: names}, names: names}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: &DomainStorageService{documents: documents, names: names}}
}

func (test *MemberEventsTest) reset(domainId string) string {
	documents := &DocumentsService{dataSource: DataSource{}}
	names := &CollectionNamesService{}
	domains := &DomainStorageService{documents: documents, names: names}
	old, _, _ := domains.Find(domainId)
	domains.Delete(old)
	(&TenantCollectionsService{documents: documents, names: names}).Drop(domainId)
	return domainId
}
