import (
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
)

// PushNotices fans a notification out to every device subscribed for the
// mail's owner, honouring the owner's per-account switches, lock-screen
// privacy, sound and action preferences. Push is opt-in: without VAPID keys,
// or with WEB_PUSH_DISABLED=true, nothing is sent.
type PushNotices struct {
	Users         *UsersHelper
	Subscriptions *PushSubscriptionsHelper
	Emails        *EmailsHelper
}

// VapidPublicKey answers the configured public key, or a throwaway one for
// local development.
func (PushNotices) VapidPublicKey() string {
	if configured := (Settings{}).Value("VAPID_PUBLIC_KEY"); configured != "" {
		return configured
	}
	public, _ := WebPush{}.GenerateKeys()
	return public
}

// NewEmail notifies of newly delivered hosted mail; it reports whether the
// work is finished (sent, or nothing to send).
func (notices PushNotices) NewEmail(email map[string]any) bool {
	return notices.notify(email, notices.newMailPayload, "notifyNewEmail", "", "")
}

// Unsnoozed notifies that a snoozed conversation came back.
func (notices PushNotices) Unsnoozed(email map[string]any) bool {
	return notices.notify(email, notices.unsnoozedPayload, "notifyUnsnoozed", "", "")
}

// ProviderNewEmail notifies of new mail in a connected provider account
// ({accountId, ownerAddress, stableId, sender, subject, receivedAt}); it
// reports whether the scheduler's outbox item is finished.
func (notices PushNotices) ProviderNewEmail(notification map[string]any) bool {
	accountId, stableId := TextValue{}.Of(notification["accountId"]), notification["stableId"]
	id := notices.externalMessageId(accountId, stableId)
	if id == "" {
		return true
	}
	owner := strings.ToLower(TextValue{}.Of(RecordValues{}.Or(notification["ownerAddress"], "")))
	domain := ""
	if parts := strings.SplitN(owner, "@", 2); len(parts) == 2 {
		domain = parts[1]
	}
	email := map[string]any{
		"id": id, "external": true, "providerAccountId": accountId, "recipient": owner, "domain": domain,
		"sender": RecordValues{}.Or(notification["sender"], ""), "subject": RecordValues{}.Or(notification["subject"], ""),
		"folder": "inbox", "systemFolder": "inbox", "read": false, "starred": false,
	}
	if receivedAt, present := notification["receivedAt"]; present {
		email["receivedAt"] = receivedAt
	}
	return notices.notify(email, notices.newMailPayload, "notifyProviderNewEmail", owner, accountId)
}

func (notices PushNotices) notify(email map[string]any, payloadFor func(map[string]any, *int, map[string]any) map[string]any, operation string, ownerEmail string, externalAccountId string) bool {
	settings := Settings{}
	if settings.Value("WEB_PUSH_DISABLED") == "true" || settings.Value("VAPID_PUBLIC_KEY") == "" || settings.Value("VAPID_PRIVATE_KEY") == "" {
		return true
	}
	sent, err := notices.deliver(email, payloadFor, ownerEmail, externalAccountId)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[push] %s failed: %s\n", operation, err.Error())
		return false
	}
	return sent
}

func (notices PushNotices) deliver(email map[string]any, payloadFor func(map[string]any, *int, map[string]any) map[string]any, ownerEmail string, externalAccountId string) (bool, error) {
	settings := Settings{}
	vapid := VapidDetails{Subject: settings.Value("VAPID_SUBJECT"), PublicKey: settings.Value("VAPID_PUBLIC_KEY"), PrivateKey: settings.Value("VAPID_PRIVATE_KEY")}
	if vapid.Subject == "" {
		vapid.Subject = "mailto:admin@caduceus.local"
	}
	lookup := ownerEmail
	if lookup == "" {
		lookup = TextValue{}.Of(email["recipient"])
	}
	_, user, err := notices.Users.FindByEmail(lookup)
	if err != nil || user == nil {
		return true, err
	}
	userEmail := TextValue{}.Of(RecordValues{}.Or(user["email"], lookup))
	domain := notices.domain(email)
	policyDomains := PreferenceChoices{}.Domains(user["domains"])
	if list, _ := (RecordValues{}).List(user["domains"]); len(list) == 0 {
		policyDomains = PreferenceChoices{}.Domains([]any{domain})
	}
	preferences := NewMailNotificationPreferences{}.Normalized(user, policyDomains)
	if externalAccountId != "" {
		preferences["newMailNotificationExternalAccounts"] = notices.externalAccounts(user, externalAccountId)
		if !(PreferenceChoices{}).Includes(preferences["newMailNotificationExternalAccounts"].([]string), externalAccountId) {
			return true, nil
		}
	} else if normalized := (Security{}).NormalizeDomain(domain); normalized == "" || !(PreferenceChoices{}).Includes(preferences["newMailNotificationDomains"].([]string), normalized) {
		return true, nil
	}
	subscriptions, err := notices.Subscriptions.ListForAddress(userEmail)
	if err != nil || len(subscriptions) == 0 {
		return true, err
	}
	var unread *int
	if externalAccountId == "" {
		unread = notices.unreadCount(user, email, userEmail)
	}
	sounds, _ := user["notificationSounds"].(map[string]any)
	preferences["soundId"] = RecordValues{}.Or(sounds["new-mail"], "chime")
	payload := TextValue{}.Json(payloadFor(email, unread, preferences))
	delivered := false
	for _, subscription := range subscriptions {
		if err := (WebPush{}).Send(subscription, payload, vapid); err == nil {
			delivered = true
		}
	}
	return delivered, nil
}

// unreadCount answers the owner's unread inbox count for the badge, or nil
// when it cannot be counted (the worker then shows a generic badge).
func (notices PushNotices) unreadCount(user map[string]any, email map[string]any, userEmail string) *int {
	unread := false
	emails, err := notices.Emails.List(notices.badgeDomains(user, notices.domain(email)), EmailListFilters{Folder: "inbox", Read: &unread, OwnerEmail: userEmail})
	if err != nil {
		fmt.Fprintln(os.Stderr, "[push] unread count failed, using generic badge:", err.Error())
		return nil
	}
	count := len(emails)
	return &count
}

// badgeDomains answers the domains the unread badge counts.
func (PushNotices) badgeDomains(user map[string]any, fallback string) []string {
	available := PreferenceChoices{}.Domains(user["domains"])
	if len(available) == 0 {
		available = PreferenceChoices{}.Domains([]any{fallback})
	}
	badge := UnreadBadgePreferences{}.Normalized(user, available)
	if badge["unreadBadgeScope"] == "domain" && badge["unreadBadgeDomain"] != "" {
		return []string{badge["unreadBadgeDomain"].(string)}
	}
	return available
}

func (PushNotices) externalAccounts(user map[string]any, accountId string) []string {
	requested, isList := RecordValues{}.List(user["newMailNotificationExternalAccounts"])
	if !isList {
		requested = []any{accountId}
	}
	chosen := []string{}
	for _, entry := range requested {
		if entry == accountId && !(PreferenceChoices{}).Includes(chosen, accountId) {
			chosen = append(chosen, accountId)
		}
	}
	return chosen
}

func (PushNotices) domain(email map[string]any) string {
	if domain := (TextValue{}).Of(email["domain"]); domain != "" {
		return strings.ToLower(domain)
	}
	parts := strings.Split(TextValue{}.Of(email["recipient"]), "@")
	return strings.ToLower(parts[len(parts)-1])
}

func (notices PushNotices) newMailPayload(email map[string]any, unread *int, preferences map[string]any) map[string]any {
	sender := RecordValues{}.String(email["sender"])
	payload := map[string]any{"title": "New mail from " + sender}
	switch (RecordValues{}).Or(preferences["newMailNotificationPrivacy"], "full") {
	case "private":
		payload = map[string]any{"title": "New mail", "body": "Open CADUCEUS to read it."}
	case "sender":
		payload["body"] = "Open CADUCEUS to read this message."
	default:
		if subject, present := email["subject"]; present {
			payload["body"] = subject
		}
	}
	soundId := notices.soundId(preferences)
	payload["data"] = map[string]any{"url": "/email/" + RecordValues{}.String(email["id"]), "emailId": email["id"], "soundId": soundId}
	payload["tag"] = "mail:" + RecordValues{}.String(email["id"])
	return notices.finished(payload, soundId, unread, preferences)
}

func (notices PushNotices) unsnoozedPayload(email map[string]any, unread *int, preferences map[string]any) map[string]any {
	soundId := notices.soundId(preferences)
	payload := map[string]any{"title": "Snoozed conversation returned", "body": "Open CADUCEUS to read it."}
	if (RecordValues{}).Or(preferences["newMailNotificationPrivacy"], "full") == "full" {
		delete(payload, "body")
		if subject, present := email["subject"]; present {
			payload["body"] = subject
		}
	}
	payload["data"] = map[string]any{"url": "/email/" + RecordValues{}.String(email["id"]), "emailId": email["id"], "reason": "snooze-expired", "soundId": soundId}
	payload["tag"] = "snooze:" + RecordValues{}.String(RecordValues{}.Or(email["threadId"], email["id"]))
	return notices.finished(payload, soundId, unread, preferences)
}

func (PushNotices) finished(payload map[string]any, soundId string, unread *int, preferences map[string]any) map[string]any {
	payload["silent"] = soundId == "none"
	if preferences["newMailNotificationActions"] != false {
		payload["actions"] = []any{
			map[string]any{"action": "mark-read", "title": "Mark read"},
			map[string]any{"action": "archive", "title": "Archive"},
		}
	}
	if unread != nil {
		payload["badgeCount"] = int(math.Max(0, float64(*unread)))
	}
	return payload
}

func (PushNotices) soundId(preferences map[string]any) string {
	if sound, isText := preferences["soundId"].(string); isText {
		return sound
	}
	return "chime"
}

// externalMessageId answers the opaque id of a provider message, or "".
func (PushNotices) externalMessageId(accountId string, stableId any) string {
	stable, isText := stableId.(string)
	if !regexp.MustCompile(`^ext_[A-Za-z0-9_-]{20,80}$`).MatchString(accountId) || !isText || stable == "" || (TextValue{}).Length(stable) > 512 || strings.ContainsAny(stable, "\x00\r\n") {
		return ""
	}
	identity := `{"accountId":` + StableJson{}.Quote(accountId) + `,"stableId":` + StableJson{}.Quote(stable) + `}`
	return "extmsg." + base64.RawURLEncoding.EncodeToString([]byte(identity))
}
