import (
	"cmp"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/ids"
	"mesail.dev/packages/notify"
	"mesail.dev/packages/outbox"
)

// @Service
type SummaryEmailService struct {
	// @Needs
	settings *NotifySettingsService
	// @Needs
	timestamps *TimestampService
	// @Needs
	summaries *SummaryStoreService
	// @Needs
	outboxStore *OutboxStoreService
	// @Needs
	relay *OutboxRelayService
}

func (service *SummaryEmailService) Send(member MemberSettingsRecord, items []NotificationRecord) (SummaryRecord, error) {
	record, failure := service.summaries.InsertIfAbsent(service.record(member, items))
	envelope, failure := service.envelopeUnless(failure, record)
	fields, failure := service.stagedUnless(failure, envelope)
	failure = service.saveUnless(failure, record, fields)
	service.relayUnless(failure, record)
	return record, failure
}

func (service *SummaryEmailService) Writes() int {
	return 4
}

func (service *SummaryEmailService) MostItems() int {
	return 20
}

func (service *SummaryEmailService) mostNameCharacters() int {
	return 200
}

func (service *SummaryEmailService) MostAddressBytes() int {
	return 318
}

func (service *SummaryEmailService) record(member MemberSettingsRecord, items []NotificationRecord) SummaryRecord {
	kept := items[:min(len(items), service.MostItems())]
	notificationIds := service.wireIds(kept)
	subject, body := notify.SummaryText{}.Build(service.cutName(member.DisplayName), service.summaryItems(kept))
	return SummaryRecord{
		DomainId:        member.DomainId,
		MemberId:        member.MemberId,
		BatchKey:        service.batchKey(notificationIds),
		Rcpt:            member.PrimaryAddress,
		Subject:         subject,
		Body:            body,
		NotificationIds: notificationIds,
		CreatedAt:       service.timestamps.Now(),
		PendingEvents:   []string{},
		DeadEvents:      []string{},
	}
}

func (service *SummaryEmailService) wireIds(items []NotificationRecord) []string {
	notificationIds := []string{}
	for _, item := range items {
		notificationIds = append(notificationIds, ids.WireId{}.Add(service.notificationPrefix(), item.Id))
	}
	return notificationIds
}

func (service *SummaryEmailService) summaryItems(items []NotificationRecord) []notify.SummaryItem {
	summaryItems := []notify.SummaryItem{}
	for _, item := range items {
		summaryItems = append(summaryItems, notify.SummaryItem{Kind: item.Kind, AuthorDisplayName: service.cutName(item.AuthorDisplayName), CreatedAt: item.CreatedAt})
	}
	return summaryItems
}

func (service *SummaryEmailService) cutName(name string) string {
	characters := []rune(name)
	return string(characters[:min(len(characters), service.mostNameCharacters())])
}

func (service *SummaryEmailService) batchKey(notificationIds []string) string {
	sorted := slices.Sorted(slices.Values(notificationIds))
	digest := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	return hex.EncodeToString(digest[:])
}

func (service *SummaryEmailService) envelopeUnless(failure error, record SummaryRecord) (events.Envelope, error) {
	if failure != nil {
		return events.Envelope{}, failure
	}
	factory := events.NewEnvelopeFactory(events.Services{}.Notify(), time.Now, rand.Reader)
	return factory.New(events.Types{}.EmailSendRequested(), record.DomainId, service.payload(record))
}

func (service *SummaryEmailService) payload(record SummaryRecord) events.EmailSendRequested {
	messageId := ids.WireId{}.Add(service.summaryPrefix(), record.Id)
	sender := cmp.Or(service.settings.Options().Summary.Sender, service.defaultSender())
	return events.EmailSendRequested{
		MessageId:      messageId,
		From:           sender + "@" + record.DomainId,
		Rcpt:           record.Rcpt,
		Role:           service.role(),
		To:             []string{record.Rcpt},
		Cc:             []string{},
		Subject:        record.Subject,
		References:     []string{},
		AttachmentIds:  []string{},
		IdempotencyKey: messageId,
	}
}

func (service *SummaryEmailService) stagedUnless(failure error, envelope events.Envelope) (outbox.Fields, error) {
	if failure != nil {
		return outbox.Fields{}, failure
	}
	settings, problem := service.settings.Outbox()
	fields, stageFailure := outbox.NewOutbox(settings).Stage(outbox.Fields{PendingEvents: []string{}, DeadEvents: []string{}}, envelope)
	return fields, cmp.Or(problem, stageFailure)
}

func (service *SummaryEmailService) saveUnless(failure error, record SummaryRecord, fields outbox.Fields) error {
	if failure != nil {
		return failure
	}
	return service.outboxStore.Save(outbox.Ref{Collection: service.summaries.Collection(record.DomainId), Id: record.Id}, fields)
}

func (service *SummaryEmailService) relayUnless(failure error, record SummaryRecord) {
	if failure != nil {
		return
	}
	_, _ = service.relay.RelayDocument(service.summaries.Collection(record.DomainId), record.Id)
}

func (service *SummaryEmailService) notificationPrefix() string {
	return "ntf"
}

func (service *SummaryEmailService) summaryPrefix() string {
	return "sum"
}

func (service *SummaryEmailService) defaultSender() string {
	return "notifications"
}

func (service *SummaryEmailService) role() string {
	return "to"
}
