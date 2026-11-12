import (
	"strings"
	"unicode/utf8"

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

func (test *SummaryEmailServiceTest) TestASummaryAtTheCapsStaysUnderTheDocumentLimit() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	emails := setup.Emails(setup.RetryLater())
	name := strings.Repeat("<", emails.mostNameCharacters())
	address := strings.Repeat("<", emails.MostAddressBytes()-len("@"+domainId)) + "@" + domainId
	member := MemberSettingsRecord{DomainId: domainId, MemberId: "member-1", EmailSummaryAfterMinutes: 15, DisplayName: name, PrimaryAddress: address}
	items := setup.StoredMany(domainId, "member-1", emails.MostItems(), name)
	record, err := emails.Send(member, items)
	stored, found, readFailure := setup.SummaryStore().Find(domainId, record.Id)
	Assert.IsTrue(err == nil && readFailure == nil && found, "summary not stored or not read back")
	Assert.IsTrue(setup.DocumentBytes(stored) < setup.DocumentLimitBytes(), "the summary document is over 512 KiB")
	Assert.Equals(record.Body, stored.Body)
	Assert.Equals(address, stored.Rcpt)
	Assert.Equals(emails.MostItems(), len(stored.NotificationIds))
	Assert.Equals(1, len(stored.PendingEvents))
	Assert.IsTrue(strings.Contains(stored.Body, "Hello "+name+","), "a name at the cap was cut")
	Assert.Equals(emails.MostItems()+1, strings.Count(stored.Body, name))
}

func (test *SummaryEmailServiceTest) TestASummaryOverTheCapsIsCut() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	emails := setup.Emails(setup.RetryLater())
	longName := strings.Repeat("\U0001F600", emails.mostNameCharacters()+1)
	cutName := strings.Repeat("\U0001F600", emails.mostNameCharacters())
	member := MemberSettingsRecord{DomainId: domainId, MemberId: "member-1", EmailSummaryAfterMinutes: 15, DisplayName: longName, PrimaryAddress: "ana@" + domainId}
	items := setup.StoredMany(domainId, "member-1", emails.MostItems()+1, longName)
	record, err := emails.Send(member, items)
	stored, found, readFailure := setup.SummaryStore().Find(domainId, record.Id)
	Assert.IsTrue(err == nil && readFailure == nil && found, "summary not stored or not read back")
	Assert.IsTrue(setup.DocumentBytes(stored) < setup.DocumentLimitBytes(), "the summary document is over 512 KiB")
	Assert.Equals(emails.MostItems(), len(stored.NotificationIds))
	Assert.Equals("You have 20 unread notifications in Mesail", stored.Subject)
	Assert.IsTrue(!strings.Contains(stored.Body, longName), "a name over the cap was stored whole")
	Assert.Equals(emails.MostItems()+1, strings.Count(stored.Body, cutName))
	Assert.IsTrue(utf8.ValidString(stored.Body), "a name was cut inside a character")
}
