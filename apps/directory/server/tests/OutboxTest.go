import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"slices"

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
	Assert.IsTrue(err == nil, "relay failed")
	Assert.IsTrue(loadErr == nil && readErr == nil, "member document not readable")
	Assert.Equals(1, state.Pending[0].Attempts)
	Assert.Equals(0, report.Delivered)
}

func (test *OutboxTest) TestAnEventThatIsNotDueIsLeftForTheNextSweep() {
	relay, _, _ := test.world()
	relay.RunOnce()
	due, err := relay.store.PendingIn("outbox-unit.test", 20)
	Assert.IsTrue(err == nil, "pending failed")
	Assert.Equals(0, len(due))
}

func (test *OutboxTest) TestAFreshEventIsDue() {
	relay, _, _ := test.world()
	due, err := relay.store.PendingIn("outbox-unit.test", 20)
	Assert.IsTrue(err == nil, "pending failed")
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
	Assert.IsTrue(err == nil, "listing failed")
	Assert.IsTrue(slices.ContainsFunc(dead, func(event DeadEvent) bool { return event.EventId == eventId }), "dead event not listed")
}

func (test *OutboxTest) TestARetriedEventIsPendingAgain() {
	relay, ref, eventId := test.world()
	err := relay.Retry(RetryRequest{Collection: ref.Collection, DocumentId: ref.Id, EventId: eventId, Subscriber: "search"})
	fields, loadErr := relay.store.Load(ref)
	Assert.IsTrue(err == nil, "retry failed")
	Assert.IsTrue(loadErr == nil, "load failed")
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

func (test *OutboxTest) world() (*OutboxRelayService, outbox.Ref, string) {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{"outbox":{"producer":"directory","subscribers":[{"name":"search","url":"http://127.0.0.1:9/api/v1/search/events"}]}}}`)
	domainId := test.reset("outbox-unit.test")
	documents := &DocumentsService{dataSource: DataSource{}}
	names := &CollectionNamesService{}
	members := &MemberStorageService{documents: documents, names: names}
	Assert.IsTrue((&DomainStorageService{documents: documents, names: names}).Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"}) == nil, "insert failed")
	staged, err := (&EventService{settings: &SettingsService{}}).StageMemberSaved(MemberDocument{MemberId: "bo-id", DomainId: domainId, DisplayName: "Bo", State: "active"})
	Assert.IsTrue(err == nil, "stage failed")
	eventId := test.unique()
	dead, _ := outbox.Codec{}.EncodeDead([]outbox.DeadLetter{{Envelope: events.Envelope{EventID: eventId, Type: events.Types{}.MemberUpserted(), DomainId: domainId, Producer: "directory"}, Subscriber: "search", Attempts: 12, LastError: "gone", DeadAt: "2026-11-10T00:00:00Z"}})
	staged.DeadEvents, staged.OutboxDead, staged.OutboxHasDead = dead, 1, true
	ref, insertErr := members.Insert(staged)
	Assert.IsTrue(insertErr == nil, "member insert failed")
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
