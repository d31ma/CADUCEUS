import (
	"net/http"
	"strings"
)

// NotificationsRoute answers /notifications/subscriptions and
// /notifications/vapid-public-key: the caller's web push devices.
type NotificationsRoute struct {
	Access        *AccessHelper
	Subscriptions *PushSubscriptionsHelper
	Push          *PushService
}

// Subscribe stores or refreshes one device subscription and answers it
// without its keys.
func (route NotificationsRoute) Subscribe(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	keys, _ := body["keys"].(map[string]any)
	values := RecordValues{}
	if !values.Truthy(body["endpoint"]) || !values.Truthy(keys["p256dh"]) || !values.Truthy(keys["auth"]) {
		return Answer{}.BadRequest("endpoint, p256dh, and auth are required")
	}
	subscription := map[string]any{
		"userEmail": caller.Email,
		"endpoint":  body["endpoint"],
		"keys":      map[string]any{"p256dh": keys["p256dh"], "auth": keys["auth"]},
	}
	if agent := request.Header.Get("User-Agent"); agent != "" {
		subscription["userAgent"] = agent
	}
	record, err := route.Subscriptions.Upsert(subscription)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(StoredRecords{}.Without(record, "keys"))
}

// Unsubscribe deletes the caller's subscription to an endpoint.
func (route NotificationsRoute) Unsubscribe(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if !(RecordValues{}).Truthy(body["endpoint"]) {
		return Answer{}.BadRequest("endpoint required")
	}
	documentId, subscription, err := route.Subscriptions.FindById(PushSubscriptionsRecords{}.SubscriptionId(TextValue{}.Of(body["endpoint"])))
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if documentId == "" || subscription == nil || subscription["userEmail"] != strings.ToLower(caller.Email) {
		return Answer{}.NotFound("Subscription not found")
	}
	if err := route.Subscriptions.DeleteDocument(documentId); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"deleted": subscription["id"]})
}

// List answers the caller's subscriptions without keys or document ids.
func (route NotificationsRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	subscriptions, err := route.Subscriptions.ListForUser(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	listed := []any{}
	for _, subscription := range subscriptions {
		listed = append(listed, StoredRecords{}.Without(subscription, "docId", "keys"))
	}
	return Answer{}.Ok(listed)
}

// VapidPublicKey answers the public key browsers subscribe with.
func (route NotificationsRoute) VapidPublicKey(request *http.Request) *http.Response {
	if _, refusal := route.Access.Signed(request); refusal != nil {
		return refusal
	}
	return Answer{}.Ok(map[string]any{"publicKey": route.Push.GetVapidPublicKey()})
}
