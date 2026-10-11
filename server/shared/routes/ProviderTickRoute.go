import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
)

// ProviderTickRoute answers /notifications/provider/tick: it leases one
// batch of provider new-mail notifications from the owner's durable outbox,
// pushes each, and settles the batch (delivered, or back to the retry
// queue). The internal HMAC is its authority.
type ProviderTickRoute struct {
	Push *PushService
}

// Tick processes one leased batch and answers {claimed, delivered, retried}.
func (route ProviderTickRoute) Tick(request *http.Request) *http.Response {
	if refusal := (WebhookSignature{}).Refusal(request, "INBOUND_WEBHOOK_SECRET"); refusal != nil {
		return refusal
	}
	accounts := ExternalAccounts{}.FromSettings()
	result, err := accounts.Call("accounts.notification_claim", map[string]any{"holder": "tachyon-" + strconv.Itoa(os.Getpid()), "limit": route.batch()})
	if err != nil {
		return accounts.Failure(err)
	}
	var claim any
	if len(result) > 0 {
		if err := json.Unmarshal(result, &claim); err != nil {
			return accounts.Failure(err)
		}
	}
	token := BodyValues{}.Field(claim, "token")
	notifications, _ := BodyValues{}.Field(claim, "notifications").([]any)
	if !(RecordValues{}).Truthy(token) || len(notifications) == 0 {
		return Answer{}.Ok(route.counts(0, 0, 0))
	}
	delivered, retried := []any{}, []any{}
	for _, entry := range notifications {
		if entry == nil {
			return accounts.Failure(nil)
		}
		notification, _ := entry.(map[string]any)
		if route.Push.NotifyProviderNewEmail(notification) {
			delivered = append(delivered, notification["id"])
		} else {
			retried = append(retried, notification["id"])
		}
	}
	if refusal := route.complete(accounts, token, delivered, true, ""); refusal != nil {
		return refusal
	}
	if refusal := route.complete(accounts, token, retried, false, "push_delivery_failed"); refusal != nil {
		return refusal
	}
	return Answer{}.Ok(route.counts(len(notifications), len(delivered), len(retried)))
}

func (ProviderTickRoute) batch() int { return 50 }

func (ProviderTickRoute) complete(accounts ExternalAccounts, token any, ids []any, delivered bool, code string) *http.Response {
	if len(ids) == 0 {
		return nil
	}
	if _, err := accounts.Call("accounts.notification_complete", map[string]any{"token": token, "ids": ids, "delivered": delivered, "code": code}); err != nil {
		return accounts.Failure(err)
	}
	return nil
}

func (ProviderTickRoute) counts(claimed int, delivered int, retried int) map[string]int {
	return map[string]int{"claimed": claimed, "delivered": delivered, "retried": retried}
}
