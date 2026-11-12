import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type OutboxTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxTest) TestTheRelayRetriesAPendingMemberEventWhileServiceTokensAreMissing() {
	relay, ref, _ := test.world()
	report, err := relay.RunOnce()
	fields, loadErr := relay.store.Load(ref)
	state, readErr := outbox.Codec{}.Read(fields)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(loadErr == nil && readErr == nil, "member document not readable")
	Assert.Equals(1, state.Pending[0].Attempts)
	Assert.Equals(0, report.Delivered)
}

func (test *OutboxTest) TestAnEventThatIsNotDueIsLeftForTheNextSweep() {
	relay, _, _ := test.world()
	relay.RunOnce()
	due, err := relay.store.PendingIn("outbox-unit.test", 20)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(0, len(due))
}

func (test *OutboxTest) TestAFreshEventIsDue() {
	relay, _, _ := test.world()
	due, err := relay.store.PendingIn("outbox-unit.test", 20)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(1, len(due))
}

func (test *OutboxTest) TestTheRelayNeedsOutboxSettings() {
	relay, _, _ := test.world()
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{}}`)
	_, err := relay.RunOnce()
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable), "want Unavailable")
}

func (test *OutboxTest) TestADeadEventIsListed() {
	relay, _, eventId := test.world()
	dead, err := relay.DeadEvents()
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(slices.ContainsFunc(dead, func(event DeadEvent) bool { return event.EventId == eventId }), "dead event not listed")
}

func (test *OutboxTest) TestARetriedEventIsPendingAgain() {
	relay, ref, eventId := test.world()
	err := relay.Retry(RetryRequest{Collection: ref.Collection, DocumentId: ref.Id, EventId: eventId, Subscriber: "search"})
	fields, loadErr := relay.store.Load(ref)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint(loadErr))
	Assert.Equals(0, fields.OutboxDead)
	Assert.IsTrue(fields.OutboxPending >= 1, "event is not pending")
}

func (test *OutboxTest) TestRetryOfAnUnknownEventIsNotFound() {
	relay, ref, _ := test.world()
	err := relay.Retry(RetryRequest{Collection: ref.Collection, DocumentId: ref.Id, EventId: "missing", Subscriber: "search"})
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *OutboxTest) TestACollectionThatIsNotAMembersCollectionIsRefused() {
	relay, ref, eventId := test.world()
	err := relay.Retry(RetryRequest{Collection: "domains", DocumentId: ref.Id, EventId: eventId, Subscriber: "search"})
	saveErr := relay.store.Save(outbox.Ref{Collection: "domains", Id: ref.Id}, outbox.Fields{})
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
	Assert.IsTrue(errors.As(saveErr, &missing), "want NotFound on save")
}

func (test *OutboxTest) TestTheOutboxRoutesRefuseAnUnverifiedCaller() {
	controller := &OutboxController{callers: &CallerService{}}
	_, relayErr := controller.PostRelay("Bearer forged")
	_, deadErr := controller.GetDead("Bearer forged")
	retryErr := controller.PostRetry("Bearer forged", RetryRequest{})
	var refused *Unauthenticated
	Assert.IsTrue(errors.As(relayErr, &refused), "relay: want Unauthenticated")
	Assert.IsTrue(errors.As(deadErr, &refused), "dead: want Unauthenticated")
	Assert.IsTrue(errors.As(retryErr, &refused), "retry: want Unauthenticated")
}

func (test *OutboxTest) TestTheDeadEventsRouteListsForAnInstallWideServiceToken() {
	relay, _, eventId := test.world()
	setup := DirectoryTestSetup{}
	realm := setup.SignedRealm()
	os.Setenv("YON_APPLICATION_JSON", setup.ApplicationJson(realm, "http://127.0.0.1:1"))
	controller := &OutboxController{callers: &CallerService{}, relay: relay}
	dead, err := controller.GetDead(setup.ServiceBearer(realm, "*"))
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(slices.ContainsFunc(dead, func(event DeadEvent) bool { return event.EventId == eventId }), "dead event not listed")
}

func (test *OutboxTest) TestTheOutboxRoutesHideThemselvesFromAMailDomainServiceToken() {
	setup := DirectoryTestSetup{}
	realm := setup.SignedRealm()
	os.Setenv("YON_APPLICATION_JSON", setup.ApplicationJson(realm, "http://127.0.0.1:1"))
	controller := &OutboxController{callers: &CallerService{}}
	_, err := controller.PostRelay(setup.ServiceBearer(realm, "outbox-unit.test"))
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *OutboxTest) TestOneRelayCallRelaysAtMostTenDocumentsAndReportsMore() {
	relay, _, _ := test.world()
	test.pendingMembers("outbox-unit.test", 11)
	report, err := relay.RunOnce()
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(report.Documents <= 10, fmt.Sprintf("one call relayed %d documents", report.Documents))
	Assert.IsTrue(report.More, "want more work reported")
}

func (test *OutboxTest) TestARelayCallStopsAfterThreeSeconds() {
	relay, _, _ := test.world()
	Assert.Equals(3*time.Second, relay.timeBudget())
	Assert.Equals(10, relay.sweepSize())
}

func (test *OutboxTest) pendingMembers(domainId string, count int) {
	members := &MemberStorageService{documents: &DocumentsService{dataSource: DataSource{}}, names: &CollectionNamesService{}}
	for index := range count {
		staged, err := (&EventService{settings: &SettingsService{}}).StageMemberSaved(MemberDocument{MemberId: fmt.Sprintf("pending-%d", index), DomainId: domainId, DisplayName: "Pending", State: "active"})
		Assert.Equals("<nil>", fmt.Sprint(err))
		_, insertErr := members.Insert(staged)
		Assert.Equals("<nil>", fmt.Sprint(insertErr))
	}
}

func (test *OutboxTest) world() (*OutboxRelayService, outbox.Ref, string) {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{"outbox":{"producer":"directory","subscribers":[{"name":"search","url":"http://127.0.0.1:9/api/v1/search/events"}]}}}`)
	domainId := test.reset("outbox-unit.test")
	documents := &DocumentsService{dataSource: DataSource{}}
	names := &CollectionNamesService{}
	members := &MemberStorageService{documents: documents, names: names}
	Assert.Equals("<nil>", fmt.Sprint((&DomainStorageService{documents: documents, names: names}).Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	staged, err := (&EventService{settings: &SettingsService{}}).StageMemberSaved(MemberDocument{MemberId: "bo-id", DomainId: domainId, DisplayName: "Bo", State: "active"})
	Assert.Equals("<nil>", fmt.Sprint(err))
	eventId := test.unique()
	dead, _ := outbox.Codec{}.EncodeDead([]outbox.DeadLetter{{Envelope: events.Envelope{EventID: eventId, Type: events.Types{}.MemberUpserted(), DomainId: domainId, Producer: "directory"}, Subscriber: "search", Attempts: 12, LastError: "gone", DeadAt: "2026-11-10T00:00:00Z"}})
	staged.DeadEvents, staged.OutboxDead, staged.OutboxHasDead = dead, 1, true
	ref, insertErr := members.Insert(staged)
	Assert.Equals("<nil>", fmt.Sprint(insertErr))
	store := &OutboxStoreService{domains: &DomainStorageService{documents: documents, names: names}, members: members, names: names}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: &DomainStorageService{documents: documents, names: names}}, ref, eventId
}

func (test *OutboxTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *OutboxTest) reset(domainId string) string {
	documents := &DocumentsService{dataSource: DataSource{}}
	names := &CollectionNamesService{}
	domains := &DomainStorageService{documents: documents, names: names}
	old, _, _ := domains.Find(domainId)
	domains.Delete(old)
	(&TenantCollectionsService{documents: documents, names: names}).Drop(domainId)
	return domainId
}
