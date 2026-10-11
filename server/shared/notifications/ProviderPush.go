import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"sync"
)

// ProviderPush delivers one scheduler-claimed provider new-mail notification
// through the same Web Push privacy, sound, action and subscription policy
// as hosted mail.
type ProviderPush struct {
	Users         *UsersHelper
	Subscriptions *PushSubscriptionsHelper
}

// Notify answers true when the durable outbox item is terminal: delivered to
// at least one device, or nothing to deliver (push off, no VAPID keys, no
// user, notifications off for the account, no subscriptions). It answers
// false when every delivery failed, so the owner retries it.
func (push ProviderPush) Notify(notification map[string]any) bool {
	accountId := notification["accountId"]
	id := ProviderMessageId{}.Of(accountId, notification["stableId"])
	if id == "" {
		return true
	}
	settings := Settings{}
	if settings.Value("WEB_PUSH_DISABLED") == "true" {
		return true
	}
	vapid := WebPushVapid{
		PublicKey: settings.Value("VAPID_PUBLIC_KEY"), PrivateKey: settings.Value("VAPID_PRIVATE_KEY"),
		Subject: settings.Value("VAPID_SUBJECT"),
	}
	if vapid.PublicKey == "" || vapid.PrivateKey == "" {
		return true
	}
	if vapid.Subject == "" {
		vapid.Subject = "mailto:admin@caduceus.local"
	}
	delivered, err := push.deliver(notification, id, vapid)
	if err != nil {
		log.Printf("[push] notifyProviderNewEmail failed: %v", err)
		return false
	}
	return delivered
}

func (push ProviderPush) deliver(notification map[string]any, id string, vapid WebPushVapid) (bool, error) {
	ownerAddress := strings.ToLower(BodyValues{}.Text(notification["ownerAddress"]))
	_, user, err := push.Users.FindByEmail(ownerAddress)
	if err != nil || user == nil {
		return user == nil && err == nil, err
	}
	userEmail := BodyValues{}.Text(RecordValues{}.Or(user["email"], ownerAddress))
	accountId, _ := notification["accountId"].(string)
	if !push.enabledFor(user, accountId) {
		return true, nil
	}
	subscriptions, err := push.Subscriptions.ListForAddress(userEmail)
	if err != nil || len(subscriptions) == 0 {
		return err == nil, err
	}
	payload, err := push.payload(notification, id, user)
	if err != nil {
		return false, err
	}
	return push.send(subscriptions, payload, vapid), nil
}

// enabledFor answers the connected-account policy: a user who never chose
// accounts gets notifications for every account they can reach.
func (ProviderPush) enabledFor(user map[string]any, accountId string) bool {
	chosen, isList := user["newMailNotificationExternalAccounts"].([]any)
	if !isList {
		return true
	}
	for _, entry := range chosen {
		if entry == accountId {
			return true
		}
	}
	return false
}

func (ProviderPush) payload(notification map[string]any, id string, user map[string]any) ([]byte, error) {
	privacy, _ := user["newMailNotificationPrivacy"].(string)
	if privacy != "sender" && privacy != "private" {
		privacy = "full"
	}
	sounds, _ := user["notificationSounds"].(map[string]any)
	soundId, isText := RecordValues{}.Or(sounds["new-mail"], "chime").(string)
	if !isText {
		soundId = "chime"
	}
	content := WebPushContent{
		Data: WebPushData{Url: "/email/" + id, EmailId: id, SoundId: soundId},
		Tag:  "mail:" + id, Silent: soundId == "none",
	}
	sender := BodyValues{}.Text(notification["sender"])
	switch privacy {
	case "private":
		content.Title, content.Body = "New mail", "Open CADUCEUS to read it."
	case "sender":
		content.Title, content.Body = "New mail from "+sender, "Open CADUCEUS to read this message."
	default:
		content.Title, content.Body = "New mail from "+sender, RecordValues{}.Or(notification["subject"], "")
	}
	if actions, isFlag := user["newMailNotificationActions"].(bool); !isFlag || actions {
		content.Actions = []WebPushAction{{Action: "mark-read", Title: "Mark read"}, {Action: "archive", Title: "Archive"}}
	}
	var written bytes.Buffer
	encoder := json.NewEncoder(&written)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(content)
	return bytes.TrimRight(written.Bytes(), "\n"), err
}

// send delivers the payload to every device at once and answers whether at
// least one push service accepted it.
func (ProviderPush) send(subscriptions []map[string]any, payload []byte, vapid WebPushVapid) bool {
	var group sync.WaitGroup
	results := make([]error, len(subscriptions))
	for index, subscription := range subscriptions {
		group.Add(1)
		go func(index int, subscription map[string]any) {
			defer group.Done()
			results[index] = WebPush{}.Send(subscription, payload, vapid)
		}(index, subscription)
	}
	group.Wait()
	for _, err := range results {
		if err == nil {
			return true
		}
	}
	return false
}
