import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type MemberEventsTest struct {
	behaviors *BehaviorContext
}

func (test *MemberEventsTest) TestJoinPublishesTheMemberEvent() {
	writer, storage, domain := test.world()
	test.join(writer, domain, "Bo Member")
	stored, _, _ := storage.Find(domain, "bo-id")
	Assert.Equals(1, len(stored.PendingEvents))
	Assert.IsTrue(strings.Contains(stored.PendingEvents[0], `"type":"member.upserted"`), "wrong event type")
	Assert.IsTrue(strings.Contains(stored.PendingEvents[0], `"domainId":"`+domain+`"`), "wrong event domain")
}

func (test *MemberEventsTest) TestANewNameIsSavedAndPublished() {
	writer, storage, domain := test.world()
	test.join(writer, domain, "Bo Member")
	test.join(writer, domain, "Bo Renamed")
	stored, _, _ := storage.Find(domain, "bo-id")
	Assert.Equals("Bo Renamed", stored.DisplayName)
	Assert.Equals(2, len(stored.PendingEvents))
}

func (test *MemberEventsTest) join(writer *MemberWriterService, domainId string, name string) {
	_, err := writer.Join(PersonDocument{PersonId: "bo-id", Username: "bo", DisplayName: name, State: "active"}, domainId, false)
	Assert.Equals("<nil>", fmt.Sprint(err))
}

func (test *MemberEventsTest) world() (*MemberWriterService, *MemberStorageService, string) {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{"outbox":{"producer":"directory","subscribers":[{"name":"search","url":"http://127.0.0.1:9/api/v1/search/events"}]}}}`)
	documents := &DocumentsService{dataSource: DataSource{}}
	members := &MemberStorageService{documents: documents, names: &CollectionNamesService{}}
	addresses := &AddressStorageService{documents: documents, names: &CollectionNamesService{}}
	return &MemberWriterService{members: members, addresses: addresses, events: &EventService{settings: &SettingsService{}}, relay: test.relay(documents)}, members, test.hosted("events-unit.test", "active")
}

func (test *MemberEventsTest) hosted(name string, state string) string {
	domainId := test.reset(name)
	documents := &DocumentsService{dataSource: DataSource{}}
	collections := &TenantCollectionsService{documents: documents, names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint((&DomainStorageService{documents: documents, names: &CollectionNamesService{}}).Insert(DomainDocument{DomainId: domainId, Name: domainId, State: state})))
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
