import (
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Test(JobsTestSetup, {"partition":"tests"})
type SummaryEmailServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SummaryEmailServiceTest) TestSendStagesTheEmailRequest() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	member := JobsTestSetup{}.MemberSettings(domainId, "member-1", 15, "ana@"+domainId)
	item := JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	emails := JobsTestSetup{}.Emails(JobsTestSetup{}.RetryLater())
	record, err := emails.Send(member, []NotificationRecord{item})
	stored, found, _ := setup.SummaryStore().Find(domainId, record.Id)
	payload := events.EmailSendRequested{}
	fields := outbox.Fields{PendingEvents: stored.PendingEvents, DeadEvents: stored.DeadEvents, OutboxPending: stored.OutboxPending, OutboxDead: stored.OutboxDead}
	envelope, published := outbox.Published{}.One(fields, events.Types{}.EmailSendRequested(), &payload)
	Assert.IsTrue(err == nil && found, "summary not stored")
	Assert.IsTrue(published == nil, "no email request staged")
	Assert.Equals("notify", envelope.Producer)
	Assert.Equals(domainId, envelope.DomainId)
	Assert.Equals("sum_"+record.Id, payload.MessageId)
	Assert.Equals("sum_"+record.Id, payload.IdempotencyKey)
	Assert.Equals("notifications@"+domainId, payload.From)
	Assert.Equals("ana@"+domainId, payload.Rcpt)
	Assert.Equals("to", payload.Role)
	Assert.Equals([]string{"ana@" + domainId}, payload.To)
	Assert.Equals("You have 1 unread notification in Mesail", payload.Subject)
	Assert.Equals([]string{"ntf_" + item.Id}, stored.NotificationIds)
}

func (test *SummaryEmailServiceTest) TestWritesCountsTheSummaryWrites() {
	Assert.Equals(4, JobsTestSetup{}.Emails(JobsTestSetup{}.RetryLater()).Writes())
}
