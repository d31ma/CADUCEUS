import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// OutboundQueue is the durable outbound delivery queue (the outbox): it
// retries transient failures with backoff (about 1m, 5m, 15m, 1h, 4h) and,
// on a permanent failure or once retries run out, bounces by dropping a
// delivery-failure notice into the sender's mailbox and suppressing the
// recipients that failed permanently. Delivery passes hold a process lock so
// overlapping worker and manual passes never claim one entry twice.
type OutboundQueue struct {
	Outbox      *OutboxHelper
	Emails      *EmailsHelper
	Scheduled   *ScheduledHelper
	Attachments *AttachmentsHelper
	Suppressed  *SuppressedHelper
	Users       *UsersHelper
	Smtp        *SmtpService
}

// Enqueue stores a message for delivery and answers its outbox id. options
// may name owner, availableAt (milliseconds), trackingId, id, scheduledId and
// attachmentEmailId.
func (queue OutboundQueue) Enqueue(sender string, email map[string]any, options map[string]any) (string, error) {
	values := RecordValues{}
	now := EpochTime{}.Now()
	availableAt, hasTime := DeliveryFields{}.Millis(options["availableAt"])
	if !hasTime {
		availableAt = now
	}
	id := values.Text(options["id"])
	if id == "" {
		id = DeliveryFields{}.Serial("out-")
	}
	owner := values.Text(options["owner"])
	if owner == "" {
		owner = sender
	}
	entry := map[string]any{
		"id": id, "sender": sender, "owner": owner, "email": email, "attempts": 0, "status": "queued",
		"nextAttempt": availableAt, "expiresAt": availableAt, "createdAt": now,
	}
	for _, field := range []string{"trackingId", "scheduledId", "attachmentEmailId"} {
		if text := values.Text(options[field]); text != "" {
			entry[field] = text
		}
	}
	_, err := queue.Outbox.Put(entry)
	return id, err
}

// ProcessDue attempts every due entry once and answers {delivered, retried,
// bounced}.
func (queue OutboundQueue) ProcessDue(now int64) (map[string]any, error) {
	result := map[string]any{}
	err := ProcessLock{Name: "outbound"}.Held(func() error {
		var err error
		result, err = queue.processDue(now)
		return err
	})
	return result, err
}

func (queue OutboundQueue) processDue(now int64) (map[string]any, error) {
	if err := queue.reconcileScheduled(); err != nil {
		return nil, err
	}
	due, err := queue.Outbox.FindDue(now)
	if err != nil {
		return nil, err
	}
	delivered, retried, bounced := 0, 0, 0
	for _, record := range due {
		entry := record.Data
		claimedAt := RecordValues{}.NumberOf(entry["claimedAt"])
		if entry["status"] == "sending" && claimedAt > float64(now-queue.lease()) {
			continue
		}
		if err := queue.Outbox.Update(record.Id, map[string]any{"status": "sending", "claimedAt": now}); err != nil {
			return nil, err
		}
		email, err := queue.hydrated(entry)
		if err == nil {
			_, err = queue.Smtp.SendEmail(RecordValues{}.Text(entry["sender"]), email)
		}
		if err == nil {
			if err := queue.Outbox.Delete(record.Id); err != nil {
				return nil, err
			}
			queue.finishScheduled(entry, "delivered")
			delivered++
			continue
		}
		outcome, err := queue.failed(record, err, now)
		if err != nil {
			return nil, err
		}
		bounced += outcome["bounced"]
		retried += outcome["retried"]
	}
	return map[string]any{"delivered": delivered, "retried": retried, "bounced": bounced}, nil
}

// failed handles one failed delivery: bounce the permanent recipients, and
// reschedule the transient ones or retire the entry.
func (queue OutboundQueue) failed(record FyloRecord, failure error, now int64) (map[string]int, error) {
	entry := record.Data
	email, _ := RecordValues{}.Object(entry["email"])
	attempts := int(RecordValues{}.NumberOf(entry["attempts"])) + 1
	permanent, transient := queue.failurePlan(email, failure, attempts >= queue.mostAttempts())
	outcome := map[string]int{"bounced": 0, "retried": 0}
	if len(permanent) > 0 {
		failedEntry := DeliveryFields{}.Merged(entry, map[string]any{"email": queue.forRecipients(email, permanent)})
		if err := queue.bounce(failedEntry, failure, attempts); err != nil {
			return nil, err
		}
		for _, recipient := range permanent {
			if err := queue.Suppressed.Suppress(recipient, "bounce"); err != nil {
				return nil, err
			}
		}
		outcome["bounced"] = 1
	}
	if len(transient) > 0 {
		patch := map[string]any{
			"status": "queued", "attempts": attempts, "nextAttempt": now + queue.backoff(attempts),
			"lastError": queue.message(failure), "claimedAt": 0, "email": queue.forRecipients(email, transient),
		}
		outcome["retried"] = 1
		return outcome, queue.Outbox.Update(record.Id, patch)
	}
	if err := queue.Outbox.Delete(record.Id); err != nil {
		return nil, err
	}
	queue.finishScheduled(entry, "failed")
	return outcome, nil
}

// Status answers secret-free queue health: depth, queued, sending, due,
// oldestAgeMs and attempts.
func (queue OutboundQueue) Status(now int64) (map[string]any, error) {
	entries, err := queue.Outbox.ListAll()
	if err != nil {
		return nil, err
	}
	values := RecordValues{}
	queued, sending, due, attempts := 0, 0, 0, 0.0
	oldest := math.Inf(1)
	for _, record := range entries {
		entry := record.Data
		if entry["status"] == "sending" {
			sending++
		} else {
			queued++
		}
		if at, isNumber := values.Number(values.Coalesce(entry["nextAttempt"], entry["expiresAt"])); isNumber && at <= float64(now) {
			due++
		}
		created := values.NumberOf(entry["createdAt"])
		if created == 0 {
			created = float64(now)
		}
		oldest = math.Min(oldest, created)
		attempts += values.NumberOf(entry["attempts"])
	}
	oldestAge := 0.0
	if len(entries) > 0 {
		oldestAge = math.Max(0, float64(now)-oldest)
	}
	return map[string]any{"depth": len(entries), "queued": queued, "sending": sending, "due": due, "oldestAgeMs": oldestAge, "attempts": attempts}, nil
}

// hydrated answers the entry's message with its bucket-stored attachments:
// queue rows store only the id of the email that owns them.
func (queue OutboundQueue) hydrated(entry map[string]any) (map[string]any, error) {
	email, _ := RecordValues{}.Object(entry["email"])
	owner := RecordValues{}.Text(entry["attachmentEmailId"])
	if owner == "" {
		return email, nil
	}
	records, err := queue.Attachments.List(owner)
	if err != nil || len(records) == 0 {
		return email, err
	}
	attachments := []any{}
	for _, record := range records {
		content, err := queue.Attachments.ReadContent(record)
		if err != nil {
			return nil, err
		}
		attachment := map[string]any{
			"filename": record["filename"], "contentType": record["contentType"],
			"contentBase64": base64.StdEncoding.EncodeToString(content), "disposition": record["disposition"],
		}
		if contentId := (RecordValues{}).Text(record["contentId"]); contentId != "" {
			attachment["contentId"] = contentId
		}
		attachments = append(attachments, attachment)
	}
	return DeliveryFields{}.Merged(email, map[string]any{"attachments": attachments}), nil
}

// finishScheduled removes the management record once delivery is terminal.
// A stale record is better than re-queuing a duplicate, so failures only log.
func (queue OutboundQueue) finishScheduled(entry map[string]any, status string) {
	scheduledId := RecordValues{}.Text(entry["scheduledId"])
	if scheduledId == "" {
		return
	}
	documentId, _, err := queue.Scheduled.FindById(scheduledId)
	if err == nil && documentId != "" {
		if err = queue.Scheduled.Update(documentId, map[string]any{"status": status, "updatedAt": (EpochTime{}).Now()}); err == nil {
			err = queue.Scheduled.Delete(documentId)
		}
	}
	if err != nil {
		fmt.Printf("[send] could not clean up scheduled metadata: %v\n", err)
	}
}

// reconcileScheduled turns schedules that have no queue entry (written
// before schedules were queued) into durable queue work.
func (queue OutboundQueue) reconcileScheduled() error {
	records, err := queue.Scheduled.ListAll()
	if err != nil {
		return err
	}
	values := RecordValues{}
	for _, record := range records {
		entry := record.Data
		if entry["transport"] == "external" || entry["status"] == "delivered" || entry["status"] == "failed" {
			continue
		}
		queueDocumentId, _, err := queue.Outbox.FindById(values.Text(values.Coalesce(entry["queueId"], entry["id"])))
		if err != nil {
			return err
		}
		if queueDocumentId != "" {
			continue
		}
		email, _ := values.Object(entry["email"])
		queueId, err := queue.Enqueue(values.Text(entry["sender"]), email, map[string]any{
			"id": entry["id"], "owner": values.Coalesce(entry["owner"], entry["sender"]), "availableAt": entry["sendAt"],
			"trackingId": email["trackingId"], "scheduledId": entry["id"], "attachmentEmailId": entry["id"],
		})
		if err != nil {
			return err
		}
		if err := queue.Scheduled.Update(record.Id, map[string]any{"queueId": queueId, "status": "scheduled", "updatedAt": (EpochTime{}).Now()}); err != nil {
			return err
		}
	}
	return nil
}

// bounce delivers a delivery-failure notice to the sender's own mailbox.
func (queue OutboundQueue) bounce(entry map[string]any, failure error, attempts int) error {
	values := RecordValues{}
	sender := values.Text(values.Coalesce(entry["owner"], entry["sender"]))
	domain := DeliveryFields{}.Domain(sender)
	email, _ := values.Object(entry["email"])
	recipients := strings.Join(queue.Recipients(email), ", ")
	daemonHost := Settings{}.Value("MAIL_HOSTNAME")
	for _, candidate := range []string{domain, "localhost"} {
		if daemonHost == "" {
			daemonHost = candidate
		}
	}
	subject := values.String(email["subject"])
	_, err := queue.Emails.Put(map[string]any{
		"id":        "bounce-" + strconv.FormatInt(EpochTime{}.Now(), 36) + "-" + DeliveryFields{}.RandomBase36(6),
		"domain":    domain,
		"recipient": sender,
		"sender":    "mailer-daemon@" + daemonHost,
		"subject":   "Undelivered: " + subject,
		"body": "Your message could not be delivered to " + recipients + " after " + strconv.Itoa(attempts) + " attempt(s).\n\n" +
			"Reason: " + queue.message(failure) + "\n\n----- Original message -----\nTo: " + recipients + "\nSubject: " + subject + "\n\n" + values.Text(email["text"]),
		"folder":     CategoryMappings{}.OfOwner(queue.Users, sender).PathFor(domain, "inbox"),
		"read":       false,
		"starred":    false,
		"receivedAt": EpochTime{}.NowText(),
		"processed":  true,
		"bounce":     true,
	})
	return err
}

// Requested answers the envelope recipients of a message as given: its
// envelopeRecipients, else to, cc and bcc.
func (OutboundQueue) Requested(email map[string]any) []string {
	fields := DeliveryFields{}
	if envelope := fields.Strings(email["envelopeRecipients"]); email["envelopeRecipients"] != nil {
		return envelope
	}
	requested := append(fields.Strings(email["to"]), fields.Strings(email["cc"])...)
	return append(requested, fields.Strings(email["bcc"])...)
}

// Recipients answers the envelope recipients without case-insensitive
// repeats (the last spelling of each wins, in first-seen order).
func (queue OutboundQueue) Recipients(email map[string]any) []string {
	order, spelling := []string{}, map[string]string{}
	for _, address := range queue.Requested(email) {
		key := strings.ToLower(address)
		if _, seen := spelling[key]; !seen {
			order = append(order, key)
		}
		spelling[key] = address
	}
	unique := []string{}
	for _, key := range order {
		unique = append(unique, spelling[key])
	}
	return unique
}

func (OutboundQueue) forRecipients(email map[string]any, recipients []string) map[string]any {
	return DeliveryFields{}.Merged(email, map[string]any{"envelopeRecipients": DeliveryFields{}.Items(recipients)})
}

// failurePlan splits a failed delivery's recipients into permanent failures
// and those to retry. A recipient an adapter result omits is unconfirmed,
// never delivered, so it is retried.
func (queue OutboundQueue) failurePlan(email map[string]any, failure error, exhausted bool) ([]string, []string) {
	requested := queue.Recipients(email)
	if exhausted {
		return requested, []string{}
	}
	var smtpFailure *SmtpFailure
	isSmtp := errors.As(failure, &smtpFailure)
	if !isSmtp || len(smtpFailure.Results) == 0 {
		if isSmtp && (SmtpDirect{}).permanent(smtpFailure.ResponseCode) {
			return requested, []string{}
		}
		return []string{}, requested
	}
	delivered, failed, permanentSet := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, result := range smtpFailure.Results {
		for _, address := range (DeliveryFields{}).Strings(result["deliveredRecipients"]) {
			delivered[strings.ToLower(address)] = true
		}
		for _, address := range (DeliveryFields{}).Strings(result["failedRecipients"]) {
			failed[strings.ToLower(address)] = true
		}
		for _, failureItem := range (SmtpDirect{}).recipientFailures(result) {
			if (SmtpDirect{}).permanent(failureItem["responseCode"]) {
				permanentSet[strings.ToLower(RecordValues{}.Text(failureItem["recipient"]))] = true
			}
		}
	}
	permanent, transient := []string{}, []string{}
	for _, address := range requested {
		key := strings.ToLower(address)
		if permanentSet[key] {
			permanent = append(permanent, address)
		} else if failed[key] || !delivered[key] {
			transient = append(transient, address)
		}
	}
	return permanent, transient
}

func (queue OutboundQueue) backoff(attempts int) int64 {
	steps := queue.backoffSteps()
	index := int(math.Min(float64(attempts), float64(len(steps)))) - 1
	return steps[int(math.Max(0, float64(index)))]
}

func (OutboundQueue) message(failure error) string {
	text := "unknown error"
	if failure != nil {
		text = failure.Error()
	}
	return TextValue{}.Prefix(regexp.MustCompile(`[\r\n\x00]+`).ReplaceAllString(text, " "), 500)
}

func (OutboundQueue) backoffSteps() []int64 {
	return []int64{60_000, 300_000, 900_000, 3_600_000, 14_400_000}
}

func (queue OutboundQueue) mostAttempts() int { return len(queue.backoffSteps()) }

// lease is how long a claimed ("sending") entry is left to its claimant.
func (OutboundQueue) lease() int64 { return 5 * 60_000 }
