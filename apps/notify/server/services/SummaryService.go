import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"
)

// @Service
type SummaryService struct {
	// @Needs
	store *NotificationStoreService
	// @Needs
	members *MemberSettingsStoreService
	// @Needs
	emails *SummaryEmailService
	// @Needs
	timestamps *TimestampService
}

func (service *SummaryService) Run(domainId string, budget JobBudget) (JobProgress, error) {
	due, failure := service.store.FindDueSummaries(domainId, time.Now().Unix(), service.readLimit())
	batches := service.batches(due)
	members, problem := service.members.ForMembers(domainId, service.memberIds(batches))
	return service.runUnless(cmp.Or(failure, problem), batches, members, budget)
}

func (service *SummaryService) batches(records []NotificationRecord) []SummaryBatch {
	grouped := service.grouped(records)
	batches := []SummaryBatch{}
	for _, memberId := range slices.Sorted(maps.Keys(grouped)) {
		ordered := service.oldestFirst(grouped[memberId])
		batches = append(batches, SummaryBatch{MemberId: memberId, Records: ordered[:min(len(ordered), service.batchLimit())]})
	}
	return batches
}

func (service *SummaryService) grouped(records []NotificationRecord) map[string][]NotificationRecord {
	grouped := map[string][]NotificationRecord{}
	for _, record := range records {
		grouped[record.MemberId] = append(grouped[record.MemberId], record)
	}
	return grouped
}

func (service *SummaryService) oldestFirst(records []NotificationRecord) []NotificationRecord {
	return slices.SortedFunc(slices.Values(records), func(first NotificationRecord, second NotificationRecord) int {
		return cmp.Or(strings.Compare(first.CreatedAt, second.CreatedAt), strings.Compare(first.Id, second.Id))
	})
}

func (service *SummaryService) memberIds(batches []SummaryBatch) []string {
	memberIds := []string{}
	for _, batch := range batches {
		memberIds = append(memberIds, batch.MemberId)
	}
	return memberIds
}

func (service *SummaryService) runUnless(failure error, batches []SummaryBatch, members map[string]MemberSettingsRecord, budget JobBudget) (JobProgress, error) {
	if failure != nil {
		return JobProgress{}, failure
	}
	return service.runAll(batches, members, budget)
}

func (service *SummaryService) runAll(batches []SummaryBatch, members map[string]MemberSettingsRecord, budget JobBudget) (JobProgress, error) {
	progress := JobProgress{}
	failures := []error{}
	for _, batch := range batches {
		step, failure := service.handle(batch, members[batch.MemberId], service.remaining(budget, progress))
		progress, failures = service.added(progress, step), append(failures, failure)
	}
	return progress, cmp.Or(failures...)
}

func (service *SummaryService) handle(batch SummaryBatch, member MemberSettingsRecord, remaining JobBudget) (JobProgress, error) {
	if remaining.Writes < len(batch.Records)+service.emails.Writes() || time.Now().After(remaining.Deadline) {
		return JobProgress{More: true}, nil
	}
	return service.summarize(batch, member)
}

func (service *SummaryService) summarize(batch SummaryBatch, member MemberSettingsRecord) (JobProgress, error) {
	summarizedAt := service.timestamps.Now()
	return service.byPreference(batch, member, summarizedAt)
}

func (service *SummaryService) byPreference(batch SummaryBatch, member MemberSettingsRecord, summarizedAt string) (JobProgress, error) {
	if member.EmailSummaryAfterMinutes > 0 && member.PrimaryAddress != "" {
		return service.send(batch, member, summarizedAt)
	}
	return service.cancel(batch)
}

func (service *SummaryService) send(batch SummaryBatch, member MemberSettingsRecord, summarizedAt string) (JobProgress, error) {
	_, failure := service.emails.Send(member, batch.Records)
	failure = service.markUnless(failure, batch.Records, summarizedAt)
	return JobProgress{Written: service.emails.Writes() + len(batch.Records), Done: 1}, failure
}

func (service *SummaryService) cancel(batch SummaryBatch) (JobProgress, error) {
	failure := service.markAll(batch.Records, "")
	return JobProgress{Written: len(batch.Records)}, failure
}

func (service *SummaryService) markUnless(failure error, records []NotificationRecord, summarizedAt string) error {
	if failure != nil {
		return failure
	}
	return service.markAll(records, summarizedAt)
}

func (service *SummaryService) markAll(records []NotificationRecord, summarizedAt string) error {
	failures := []error{}
	for _, record := range records {
		failures = append(failures, service.store.MarkSummarized(record, summarizedAt))
	}
	return cmp.Or(failures...)
}

func (service *SummaryService) remaining(budget JobBudget, progress JobProgress) JobBudget {
	return JobBudget{Writes: budget.Writes - progress.Written, Deadline: budget.Deadline}
}

func (service *SummaryService) added(total JobProgress, step JobProgress) JobProgress {
	return JobProgress{Written: total.Written + step.Written, Done: total.Done + step.Done, More: total.More || step.More}
}

func (service *SummaryService) readLimit() int {
	return 200
}

func (service *SummaryService) batchLimit() int {
	return 20
}
