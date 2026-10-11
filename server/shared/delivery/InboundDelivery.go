import (
	"log"
	"strings"
)

// InboundDelivery runs the inbound pipeline for one received message:
// domain check, route match, spam classification, then storing the source,
// attachments and email in the domain's tenant storage, the route action,
// and for legitimate mail the owner's inbox rules, a push notice, follow-up
// reminder completion and found-in-mail contacts. Calendar invitations are
// not imported here: the calendar area owns that import.
type InboundDelivery struct {
	Emails       *EmailsHelper
	Domains      *DomainsHelper
	Users        *UsersHelper
	Routing      InboundRouting
	Spam         *SpamFilterService
	FollowUps    FollowUpReminders
	Participants MailParticipants
	Notices      PushNotices
	Storage      TenantStorage
}

// Deliver answers the route answer for one signed delivery payload
// ({recipient, sender, subject?, body?, messageId?, spamVerdict?,
// spamScore?}).
func (delivery InboundDelivery) Deliver(payload map[string]any) (map[string]any, error) {
	values := RecordValues{}
	if !values.Truthy(payload["recipient"]) || !values.Truthy(payload["sender"]) {
		return map[string]any{"error": "recipient and sender required"}, nil
	}
	recipient, sender := strings.ToLower(values.String(payload["recipient"])), strings.ToLower(values.String(payload["sender"]))
	subject := values.Text(values.Coalesce(payload["subject"], "(no subject)"))
	messageId := values.Text(values.Coalesce(payload["messageId"], RandomText{}.Hex(16)))
	raw := values.Text(payload["body"])
	parsed := InboundMime{}.Parsed(raw)
	domain := DeliveryFields{}.Domain(recipient)
	threadDomain := domain
	if threadDomain == "" {
		threadDomain = "localhost"
	}
	metadata := Threading{}.ThreadMetadataFromHeaders(parsed.Headers, messageId, threadDomain)
	if domain == "" {
		return map[string]any{"error": "Invalid recipient address"}, nil
	}
	_, config, err := delivery.Domains.FindEntry(domain)
	if err != nil {
		return nil, err
	}
	if config == nil || config["inboundEnabled"] != true {
		return map[string]any{"accepted": false, "reason": "domain not configured"}, nil
	}
	_, user, err := delivery.Users.FindByEmail(recipient)
	if err != nil {
		return nil, err
	}
	ownerEmail := values.Text(values.Or(user["email"], recipient))
	route := delivery.Routing.MatchRoute(config["routes"], recipient)
	action, _ := values.Object(route["action"])
	if route == nil || action["type"] == "drop" {
		return map[string]any{"accepted": false, "reason": "dropped by route rule"}, nil
	}
	spam := delivery.Spam.Classified(map[string]any{
		"sender": sender, "subject": subject, "text": parsed.Text, "headers": parsed.Headers,
		"verdict": payload["spamVerdict"], "score": payload["spamScore"],
	})
	message := InboundMessage{
		Id: messageId, Domain: domain, Recipient: recipient, Sender: sender, Owner: ownerEmail, Subject: subject,
		Raw: raw, Parsed: parsed, Metadata: metadata, Spam: spam,
	}
	var result map[string]any
	err = delivery.Storage.Within(domain, func() error {
		var err error
		result, err = delivery.accepted(message, route)
		return err
	})
	return result, err
}

func (delivery InboundDelivery) accepted(message InboundMessage, route map[string]any) (map[string]any, error) {
	if err := delivery.store(message); err != nil {
		return nil, err
	}
	if err := delivery.Routing.ApplyRouteAction(route, message.Id, message.Sender, message.Recipient, message.Subject); err != nil {
		return nil, err
	}
	isSpam := message.Spam["isSpam"] == true
	if !isSpam {
		if err := delivery.legitimate(message); err != nil {
			return nil, err
		}
	}
	result := map[string]any{"accepted": true, "emailId": message.Id}
	if isSpam {
		result["spam"], result["spamScore"] = true, message.Spam["score"]
	}
	return result, nil
}

// store saves the source, attachments and email; a failure removes the
// files it saved.
func (delivery InboundDelivery) store(message InboundMessage) error {
	attachments := delivery.Emails.attachmentsHelper
	raw := delivery.Emails.rawMessagesHelper
	if _, err := raw.SaveForEmail(message.Id, message.Domain, []byte(message.Raw)); err != nil {
		return err
	}
	_, err := attachments.SaveForEmail(message.Id, message.Domain, message.Parsed.Attachments)
	if err == nil {
		_, err = delivery.Emails.Put(delivery.email(message))
	}
	if err != nil {
		_ = attachments.DeleteForEmail(message.Id)
		_ = raw.DeleteForEmail(message.Id)
	}
	return err
}

func (delivery InboundDelivery) email(message InboundMessage) map[string]any {
	fields := DeliveryFields{}
	security := PhishingDetection{}.Analyzed(message.Sender, message.Parsed.Headers)
	unsubscribe := UnsubscribeLinks{}.Parsed(message.Parsed.Headers, message.Parsed.Html)
	recipients := PersonalLevel{}.RecipientsFromHeaders(message.Parsed.Headers, message.Recipient)
	roles := CategoryRoles{Users: delivery.Users}
	folder := roles.PathFor(message.Owner, message.Domain, "inbox")
	if message.Spam["isSpam"] == true {
		folder = roles.PathFor(message.Owner, message.Domain, "spam")
	}
	email := map[string]any{
		"id": message.Id, "domain": message.Domain, "recipient": message.Recipient, "ownerEmail": message.Owner, "sender": message.Sender,
		"subject": message.Subject, "body": message.Parsed.Text, "folder": folder, "read": false, "starred": false,
		"receivedAt": EpochTime{}.NowText(), "processed": false,
	}
	if from := (RecordValues{}).Text(security["headerFrom"]); from != "" {
		email["headerFrom"] = from
	}
	for _, field := range []string{"replyTo", "securityWarnings"} {
		if list := security[field].([]string); len(list) > 0 {
			email[field] = fields.Items(list)
		}
	}
	for _, field := range []string{"unsubscribeUrl", "unsubscribeMailto", "unsubscribeBodyUrl"} {
		if text := (RecordValues{}).Text(unsubscribe[field]); text != "" {
			email[field] = text
		}
	}
	if unsubscribe["unsubscribeOneClick"] == true {
		email["unsubscribeOneClick"] = true
	}
	for _, field := range []string{"to", "cc", "bcc"} {
		if len(recipients[field]) > 0 {
			email[field] = fields.Items(recipients[field])
		}
	}
	if message.Parsed.Html != "" {
		email["htmlBody"] = message.Parsed.Html
	}
	if score := (RecordValues{}).NumberOf(message.Spam["score"]); score != 0 {
		email["spamScore"] = score
	}
	if reasons := message.Spam["reasons"].([]string); len(reasons) > 0 {
		email["spamReasons"] = fields.Items(reasons)
	}
	return fields.Merged(email, message.Metadata)
}

// legitimate runs the steps only mail outside the spam folder gets.
func (delivery InboundDelivery) legitimate(message InboundMessage) error {
	recipients := PersonalLevel{}.RecipientsFromHeaders(message.Parsed.Headers, message.Recipient)
	delivery.Routing.ApplyInboxRules(message.Id, message.Owner, message.Domain, map[string]any{
		"sender": message.Sender, "recipient": message.Recipient, "to": recipients["to"], "cc": recipients["cc"],
		"subject": message.Subject, "body": message.Parsed.Text, "hasAttachment": len(message.Parsed.Attachments) > 0,
	})
	_, delivered, err := delivery.Emails.FindById(message.Id)
	if err != nil {
		return err
	}
	if delivered != nil {
		delivery.Notices.NewEmail(delivered)
	}
	threadId := RecordValues{}.Text(message.Metadata["threadId"])
	if _, err := delivery.FollowUps.ResolveForReply(message.Owner, message.Sender, message.Subject, threadId); err != nil {
		log.Printf("[inbound] follow-up reminder resolution failed: %v", err)
		return err
	}
	delivery.Participants.Observe(message.Owner, []string{message.Sender}, message.Domain)
	return nil
}
