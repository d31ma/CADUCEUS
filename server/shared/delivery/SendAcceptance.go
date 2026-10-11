import (
	"errors"
	"log"
	"math"
	"strings"
	"unicode/utf8"
)

// SendAcceptance answers POST /send: it validates and authorizes one
// outbound message, then delivers it now, after an undo delay, or at a
// scheduled time, through the hosted queue or the caller's connected
// account, with an optional follow-up reminder, Send & Archive, and an
// exactly-once idempotency key.
type SendAcceptance struct {
	Sent       SentMail
	Journal    SendIdempotency
	Reminders  FollowUpReminders
	Archive    ConversationArchive
	Suppressed *SuppressedHelper
	Senders    OwnedSenders
	Limits     *RateLimitService
}

// Accept answers the route answer for a signed-in caller's send body.
func (send SendAcceptance) Accept(caller RouteCaller, body map[string]any, ipAddress string) (any, error) {
	if body == nil {
		body = map[string]any{}
	}
	plan := SendPlan{Caller: caller}
	requestHash := ""
	if body["idempotencyKey"] != nil {
		key, _ := body["idempotencyKey"].(string)
		if count := utf8.RuneCountInString(key); count < 1 || count > 128 || strings.ContainsAny(key, "\r\n\x00") {
			return map[string]any{"error": "idempotencyKey must be 1-128 safe characters"}, nil
		}
		plan.IdempotencyKey, requestHash = key, send.Journal.Hash(body)
		kind, replay, err := send.Journal.Inspect(caller.Email, key, requestHash)
		if err != nil {
			return send.refused(err)
		}
		if kind == "replay" {
			return replay, nil
		}
	}
	limit, err := send.Limits.Check([]string{"send", caller.Email, ipAddress}, send.hourlyLimit(), send.hour())
	if err != nil {
		return nil, err
	}
	if !limit.Allowed {
		return map[string]any{"error": "Too many send attempts", "retryAfterSeconds": limit.RetryAfterSeconds, "rateLimited": true}, nil
	}
	if answer, err := send.prepare(&plan, body); answer != nil || err != nil {
		return answer, err
	}
	if plan.IdempotencyKey != "" {
		kind, value, err := send.Journal.Begin(caller.Email, plan.IdempotencyKey, requestHash)
		if err != nil {
			return send.refused(err)
		}
		if kind == "replay" {
			return value, nil
		}
		plan.JournalId, _ = value.(string)
	}
	if plan.Account != nil {
		return ProviderSend{Acceptance: send}.Accept(plan)
	}
	result, err := send.hosted(plan)
	if err != nil {
		send.Journal.MarkAmbiguous(plan.JournalId)
		return nil, err
	}
	return send.Journal.Accept(plan.JournalId, result)
}

// refused answers the 409 of an idempotency conflict; other errors pass on.
func (SendAcceptance) refused(err error) (any, error) {
	var refusal *DeliveryRefusal
	if errors.As(err, &refusal) {
		return refusal.Answer(), nil
	}
	return nil, err
}

// prepare validates the body into the plan, answering the refusal when the
// body cannot be sent.
func (send SendAcceptance) prepare(plan *SendPlan, body map[string]any) (any, error) {
	request, problem := MailValidation{}.SendRequest(body)
	if problem != "" {
		return map[string]any{"error": problem}, nil
	}
	attachments, problem := MailValidation{}.Attachments(body["attachments"])
	if problem != "" {
		return map[string]any{"error": problem}, nil
	}
	if answer, err := send.sender(plan, request); answer != nil || err != nil {
		return answer, err
	}
	domain := DeliveryFields{}.Domain(plan.Sender)
	if domain == "" {
		domain = "localhost"
	}
	request["messageId"] = Threading{}.CreateMessageId(domain, "")
	if len(attachments) > 0 {
		request["attachments"] = attachments
	}
	plan.Request = request
	fields := DeliveryFields{}
	plan.Recipients = append(append(fields.Strings(request["to"]), fields.Strings(request["cc"])...), fields.Strings(request["bcc"])...)
	suppressed, err := send.Suppressed.AsSet()
	if err != nil {
		return nil, err
	}
	blocked := []string{}
	for _, address := range plan.Recipients {
		if suppressed[address] {
			blocked = append(blocked, address)
		}
	}
	if len(blocked) > 0 {
		return map[string]any{"error": "Recipients are suppressed", "blocked": blocked}, nil
	}
	if answer, err := send.archiveSource(plan, body); answer != nil || err != nil {
		return answer, err
	}
	if request["openTracking"] == true || request["linkTracking"] == true {
		plan.TrackingId = fields.UuidSeven()
	}
	if answer := send.timing(plan, body); answer != nil {
		return answer, nil
	}
	if delay, isNumber := body["delayMs"].(float64); isNumber && delay > 0 {
		plan.DelayMs = int64(delay)
	}
	return nil, nil
}

// sender resolves the From address: an owned address, else a send-capable
// connected account with that address.
func (send SendAcceptance) sender(plan *SendPlan, request map[string]any) (any, error) {
	from, _ := request["from"].(string)
	sender, err := send.Senders.Sender(plan.Caller.Email, request["from"])
	if err != nil {
		return nil, err
	}
	if sender == "" && from != "" {
		accounts, err := DeliveryAccounts{Token: plan.Caller.Token}.List()
		if err != nil {
			return DeliveryAccounts{}.Failure(err), nil
		}
		for _, account := range accounts {
			capabilities, _ := RecordValues{}.Object(account["capabilities"])
			state, _ := account["state"].(string)
			if account["mailboxAddress"] == from && capabilities["send"] == true && strings.Contains(" ready syncing idle degraded ", " "+state+" ") {
				plan.Account, sender = account, from
				break
			}
		}
	}
	if sender == "" {
		return map[string]any{"error": "From address is not owned by this mailbox", "forbidden": true}, nil
	}
	plan.Sender = sender
	return nil, nil
}

// archiveSource checks Send & Archive and plans the source archive.
func (send SendAcceptance) archiveSource(plan *SendPlan, body map[string]any) (any, error) {
	if _, isBool := body["archive"].(bool); body["archive"] != nil && !isBool {
		return map[string]any{"error": "archive must be a boolean"}, nil
	}
	if body["archiveSourceId"] != nil && body["archive"] != true {
		return map[string]any{"error": "archiveSourceId requires archive: true"}, nil
	}
	if body["archive"] != true {
		return nil, nil
	}
	sourceText, _ := body["archiveSourceId"].(string)
	source := Security{}.Trim(sourceText)
	if source == "" {
		return map[string]any{"error": "archiveSourceId is required for Send & Archive"}, nil
	}
	if body["sendAt"] != nil {
		return map[string]any{"error": "Send & Archive cannot be combined with scheduled delivery"}, nil
	}
	if (ExternalSubmission{}).ProviderMessage(source) {
		plan.ArchiveSourceId = source
		return nil, nil
	}
	archivePlan, err := send.Archive.Plan(source, plan.Caller.Email, plan.Caller.Domains)
	var refusal *DeliveryRefusal
	if errors.As(err, &refusal) {
		return refusal.Answer(), nil
	}
	plan.ArchivePlan = archivePlan
	return nil, err
}

// timing checks remindAt and sendAt.
func (send SendAcceptance) timing(plan *SendPlan, body map[string]any) any {
	now := float64(EpochTime{}.Now())
	if body["remindAt"] != nil {
		text, isText := body["remindAt"].(string)
		if !isText {
			return map[string]any{"error": "remindAt must be an ISO datetime"}
		}
		remindAt := EpochTime{}.Parse(text)
		if math.IsNaN(remindAt) {
			return map[string]any{"error": "remindAt must be a valid ISO datetime"}
		}
		if remindAt <= now {
			return map[string]any{"error": "remindAt must be in the future"}
		}
		plan.RemindAt, plan.HasReminder = int64(remindAt), true
	}
	if body["sendAt"] != nil {
		text, isText := body["sendAt"].(string)
		if !isText {
			return map[string]any{"error": "sendAt must be an ISO datetime"}
		}
		sendAt := EpochTime{}.Parse(text)
		if math.IsNaN(sendAt) {
			return map[string]any{"error": "sendAt must be a valid ISO datetime"}
		}
		if sendAt <= now {
			return map[string]any{"error": "sendAt must be in the future"}
		}
		if plan.HasReminder && float64(plan.RemindAt) <= sendAt {
			return map[string]any{"error": "remindAt must be after sendAt"}
		}
		plan.SendAt, plan.HasSendAt = int64(sendAt), true
	}
	return nil
}

// hosted delivers through the hosted outbound queue.
func (send SendAcceptance) hosted(plan SendPlan) (any, error) {
	caller := plan.Caller
	if plan.HasSendAt {
		scheduled, err := send.Sent.Schedule(caller.Email, plan.Sender, plan.Request, plan.SendAt, plan.TrackingId)
		if err != nil {
			return nil, err
		}
		return send.WithReminder(plan, scheduled)
	}
	var accepted map[string]any
	var err error
	if plan.DelayMs > 0 {
		accepted, err = send.Sent.Delay(caller.Email, plan.Sender, plan.Request, plan.DelayMs, plan.TrackingId)
	} else {
		accepted, err = send.Sent.SendNow(caller.Email, plan.Sender, plan.Request, plan.TrackingId)
	}
	if err != nil {
		return nil, err
	}
	result, err := send.WithReminder(plan, accepted)
	if err != nil || plan.ArchivePlan == nil {
		return result, err
	}
	archived, err := send.archive(plan, accepted)
	if err != nil {
		log.Printf("[send] message accepted but source archive failed: %v", err)
		archived = map[string]any{"archived": false, "archivedCount": 0, "archiveError": "Source conversation could not be archived"}
	}
	return DeliveryFields{}.Merged(result, archived), nil
}

func (send SendAcceptance) archive(plan SendPlan, accepted map[string]any) (map[string]any, error) {
	if undoId := (RecordValues{}).Text(accepted["undoId"]); undoId != "" {
		if err := send.Sent.AttachArchiveRollback(undoId, send.Archive.Serialized(plan.ArchivePlan)); err != nil {
			return nil, err
		}
	}
	return send.Archive.Archive(plan.ArchivePlan, plan.Caller.Email)
}

// WithReminder schedules the plan's follow-up reminder, if any, on the
// accepted message and adds its reminderId.
func (send SendAcceptance) WithReminder(plan SendPlan, result map[string]any) (map[string]any, error) {
	if !plan.HasReminder {
		return result, nil
	}
	values := RecordValues{}
	emailId := values.Coalesce(values.Coalesce(result["messageId"], result["undoId"]), result["id"])
	reminder, err := send.Reminders.Schedule(map[string]any{
		"ownerEmail": plan.Caller.Email, "emailId": emailId, "recipient": strings.Join(DeliveryFields{}.Strings(plan.Request["to"]), ", "),
		"subject": plan.Request["subject"], "threadId": result["threadId"], "remindAt": plan.RemindAt,
	})
	if err != nil {
		return nil, err
	}
	return DeliveryFields{}.Merged(result, map[string]any{"reminderId": reminder["id"]}), nil
}

// IpAddress answers the caller's address as the former RouteRequest read
// it: the first X-Forwarded-For entry, else X-Real-IP.
func (SendAcceptance) IpAddress(forwardedFor string, realIp string) string {
	if first := strings.TrimSpace(strings.Split(forwardedFor, ",")[0]); first != "" {
		return first
	}
	return realIp
}

func (SendAcceptance) hourlyLimit() int { return 60 }

func (SendAcceptance) hour() int64 { return 60 * 60 * 1000 }
