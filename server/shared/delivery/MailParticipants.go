import (
	"log"
	"strings"
)

// MailParticipants learns "found in mail" contacts from the people a mailbox
// writes to and hears from, as the former ContactsService.observeParticipants
// did: a known contact's interaction count moves on, an unknown address
// becomes a mail-sourced contact. Contacts owns the full service; this is
// the minimal private version delivery needs.
type MailParticipants struct {
	Contacts *ContactsHelper
	Users    *UsersHelper
}

// Observe records the addresses for the owner, unless the owner turned
// found-in-mail learning off for the mailbox domain. Failures only log.
func (participants MailParticipants) Observe(ownerEmail string, addresses []string, mailboxDomain string) {
	security := Security{}
	if mailboxDomain == "" {
		mailboxDomain = DeliveryFields{}.Domain(ownerEmail)
	}
	domain := security.NormalizeDomain(mailboxDomain)
	if domain != "" {
		_, owner, _ := participants.Users.FindByEmail(ownerEmail)
		preferences, _ := RecordValues{}.Object(owner["mailPreferences"])
		if (TextValue{}).Includes(DeliveryFields{}.Strings(preferences["findInMailDisabledDomains"]), domain) {
			return
		}
	}
	seen := map[string]bool{}
	for _, raw := range addresses {
		email := security.NormalizeEmailAddress(raw)
		if seen[raw] || email == "" || email == strings.ToLower(ownerEmail) {
			seen[raw] = true
			continue
		}
		seen[raw] = true
		if err := participants.observe(ownerEmail, email); err != nil {
			log.Printf("[contacts] found-in-mail update failed: %v", err)
		}
	}
}

func (participants MailParticipants) observe(ownerEmail string, email string) error {
	existing, err := participants.Contacts.FindByEmail(ownerEmail, email)
	if err != nil {
		return err
	}
	now := EpochTime{}.NowText()
	if existing != nil {
		contact := StoredRecords{}.Without(existing, "docId")
		contact["interactionCount"] = RecordValues{}.NumberOf(existing["interactionCount"]) + 1
		contact["lastContactedAt"], contact["updatedAt"] = now, now
		return participants.Contacts.Replace(RecordValues{}.Text(existing["docId"]), contact)
	}
	owner := strings.ToLower(ownerEmail)
	_, err = participants.Contacts.Put(map[string]any{
		"id": "contact-" + Security{}.RandomUuid(), "ownerEmail": owner, "domain": DeliveryFields{}.Domain(owner),
		"source": "mail", "name": "", "givenName": "", "familyName": "", "company": "", "title": "",
		"emails": []any{map[string]any{"value": email}}, "phones": []any{}, "addresses": []any{}, "urls": []any{},
		"nicknames": []any{}, "birthday": "", "notes": "", "groupIds": []any{},
		"interactionCount": 1, "lastContactedAt": now, "createdAt": now, "updatedAt": now,
	})
	return err
}
