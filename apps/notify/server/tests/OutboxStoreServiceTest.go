import (
	"errors"

	"mesail.dev/packages/outbox"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type OutboxStoreServiceTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxStoreServiceTest) stored() (outbox.Ref, SummaryRecord) {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	record, err := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	if err != nil {
		panic(err)
	}
	return outbox.Ref{Collection: setup.SummaryStore().Collection(domainId), Id: record.Id}, record
}

func (test *OutboxStoreServiceTest) TestLoadReadsTheFourFields() {
	ref, record := test.stored()
	fields, err := NotifyTestSetup{}.OutboxStore().Load(ref)
	Assert.IsTrue(err == nil, "load failed")
	Assert.Equals(record.OutboxPending, fields.OutboxPending)
	Assert.Equals(1, len(fields.PendingEvents))
}

func (test *OutboxStoreServiceTest) TestLoadRefusesAnUnregisteredCollection() {
	_, err := NotifyTestSetup{}.OutboxStore().Load(outbox.Ref{Collection: NotifyTestSetup{}.Name("other"), Id: "x"})
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}

func (test *OutboxStoreServiceTest) TestSaveSettlesTheDocument() {
	ref, _ := test.stored()
	store := NotifyTestSetup{}.OutboxStore()
	err := store.Save(ref, outbox.Fields{PendingEvents: []string{}, DeadEvents: []string{"{}"}, OutboxPending: 0, OutboxDead: 1})
	fields, _ := store.Load(ref)
	Assert.IsTrue(err == nil, "save failed")
	Assert.Equals(0, fields.OutboxPending)
	Assert.Equals(1, fields.OutboxDead)
}

func (test *OutboxStoreServiceTest) TestPendingFindsAStagedDocument() {
	NotifyTestSetup{}.FreshRegistries()
	ref, _ := test.stored()
	documents, err := NotifyTestSetup{}.OutboxStore().Pending(100000)
	Assert.IsTrue(err == nil, "pending failed")
	Assert.IsTrue(test.contains(documents, ref), "staged document missing")
}

func (test *OutboxStoreServiceTest) TestDeadFindsADeadDocument() {
	NotifyTestSetup{}.FreshRegistries()
	ref, _ := test.stored()
	store := NotifyTestSetup{}.OutboxStore()
	store.Save(ref, outbox.Fields{PendingEvents: []string{}, DeadEvents: []string{"{}"}, OutboxPending: 0, OutboxDead: 1})
	documents, err := store.Dead(100000)
	Assert.IsTrue(err == nil, "dead failed")
	Assert.IsTrue(test.contains(documents, ref), "dead document missing")
}

func (test *OutboxStoreServiceTest) contains(documents []outbox.Document, ref outbox.Ref) bool {
	for _, document := range documents {
		if document.Ref == ref {
			return true
		}
	}
	return false
}
