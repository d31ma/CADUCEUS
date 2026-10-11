import (
	"bytes"
	"log"
	"net/http"
	"strings"
	"time"
)

// InboundRouting applies a domain's routes and a mailbox's inbox rules to
// delivered mail: the first enabled route matching the recipient decides
// whether mail is stored, forwarded, posted to a webhook or dropped, and the
// owner's inbox rules then move, mark, star or forward the stored copy.
type InboundRouting struct {
	Emails *EmailsHelper
	Rules  *RulesHelper
	Users  *UsersHelper
	Smtp   *SmtpService
}

// MatchRoute answers the first enabled route matching the recipient, or nil.
func (InboundRouting) MatchRoute(routes any, recipient string) map[string]any {
	listed, _ := RecordValues{}.List(routes)
	for _, item := range listed {
		route, _ := RecordValues{}.Object(item)
		if route == nil || !(RecordValues{}).Truthy(route["enabled"]) {
			continue
		}
		match := RecordValues{}.Text(route["match"])
		if match == "*" || match == "*@"+(DeliveryFields{}).Domain(recipient) || match == recipient {
			return route
		}
	}
	return nil
}

// ApplyRouteAction runs a route's webhook or forward for an accepted
// message whose body is not needed.
func (routing InboundRouting) ApplyRouteAction(route map[string]any, messageId string, sender string, recipient string, subject string) error {
	action, _ := RecordValues{}.Object(route["action"])
	switch action["type"] {
	case "webhook":
		payload := StoredRecords{}.JsonText(map[string]any{"messageId": messageId, "sender": sender, "recipient": recipient, "subject": subject})
		client := http.Client{Timeout: 30 * time.Second}
		response, err := client.Post(RecordValues{}.Text(action["url"]), "application/json", bytes.NewReader([]byte(payload)))
		if err != nil {
			log.Printf("[inbound] route webhook action failed: %v", err)
			return nil
		}
		response.Body.Close()
	case "forward":
		if to := (RecordValues{}).Text(action["to"]); to != "" && to != recipient {
			_, err := routing.Smtp.Send(map[string]any{"to": []any{to}, "subject": "Fwd: " + subject, "text": "Forwarded from " + sender + " to " + recipient})
			return err
		}
	}
	return nil
}

// ApplyInboxRules applies the owner's enabled inbox rules to a stored
// message; failures only log.
func (routing InboundRouting) ApplyInboxRules(messageId string, ownerEmail string, domain string, meta map[string]any) {
	rules, err := routing.Rules.ListEnabledForMailbox(ownerEmail, RecordValues{}.Text(meta["recipient"]), domain)
	if err != nil {
		log.Printf("[inbound] applyInboxRules failed: %v", err)
		return
	}
	for _, rule := range rules {
		matched, stop := routing.evaluated(rule, meta)
		if !matched {
			continue
		}
		if err := routing.applyActions(rule, messageId, meta); err != nil {
			log.Printf("[inbound] applyInboxRules failed: %v", err)
			return
		}
		if stop {
			break
		}
	}
}

func (routing InboundRouting) applyActions(rule map[string]any, messageId string, meta map[string]any) error {
	documentId, _, err := routing.Emails.FindById(messageId)
	if err != nil || documentId == "" {
		return err
	}
	values := RecordValues{}
	patch := map[string]any{}
	actions, _ := values.List(rule["actions"])
	for _, item := range actions {
		action, _ := values.Object(item)
		switch action["type"] {
		case "folder":
			patch["folder"] = action["folder"]
		case "delete":
			recipient := values.Text(meta["recipient"])
			patch["folder"] = CategoryMappings{}.OfOwner(routing.Users, recipient).PathFor(DeliveryFields{}.Domain(recipient), "trash")
		case "markRead":
			patch["read"] = action["value"]
		case "star":
			patch["starred"] = action["value"]
		case "forward":
			if _, err := routing.Smtp.Send(map[string]any{"to": []any{action["to"]}, "subject": "Fwd: " + values.Text(meta["subject"]), "text": "Forwarded"}); err != nil {
				log.Printf("[inbound] inbox rule forward failed: %v", err)
			}
		}
	}
	if len(patch) == 0 {
		return nil
	}
	return routing.Emails.Update(documentId, patch)
}

// evaluated answers whether an inbox rule matches the message, and whether
// it stops later rules (the former RuleEngine.evaluateInboxRule, which the
// rules area owns; this is the minimal private version delivery needs).
func (routing InboundRouting) evaluated(rule map[string]any, meta map[string]any) (bool, bool) {
	values := RecordValues{}
	message := map[string]string{
		"from": routing.text(values.Coalesce(meta["from"], meta["sender"])), "to": routing.text(values.Coalesce(meta["to"], meta["recipient"])),
		"cc": routing.text(meta["cc"]), "subject": routing.text(meta["subject"]), "body": routing.text(meta["body"]),
	}
	conditions, _ := values.List(rule["conditions"])
	matched := len(conditions) == 0
	all := rule["conditionMatch"] == "all"
	if all {
		matched = true
	}
	for _, item := range conditions {
		condition, _ := values.Object(item)
		result := InboxRuleEngine{}.Matches(condition, message, meta["hasAttachment"] == true)
		if all {
			matched = matched && result
		} else {
			matched = matched || result
		}
	}
	return matched, matched && values.Truthy(rule["stopProcessing"])
}

func (InboundRouting) text(value any) string {
	if list, isList := (RecordValues{}).List(value); isList {
		parts := []string{}
		for _, item := range list {
			if item == nil {
				parts = append(parts, "")
			} else {
				parts = append(parts, RecordValues{}.String(item))
			}
		}
		return strings.Join(parts, ", ")
	}
	if value == nil {
		return ""
	}
	return RecordValues{}.String(value)
}
