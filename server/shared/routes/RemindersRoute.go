import (
	"math"
	"net/http"
	"strings"
)

// RemindersRoute answers /reminders, the caller's follow-up reminders, and
// /reminders/tick, the signed system pass over every mailbox.
type RemindersRoute struct {
	Access    *AccessHelper
	Emails    *EmailsHelper
	Reminders *FollowUpReminderService
}

// List answers GET /reminders.
func (route RemindersRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	reminders, err := route.Reminders.Reminders().ListForOwner(caller.Email)
	if err != nil {
		return DeliveryReply{}.Settled(nil, err)
	}
	listed := []any{}
	for _, reminder := range reminders {
		item := StoredRecords{}.Without(reminder, "docId")
		item["remindAt"] = route.instant(item["remindAt"])
		item["createdAt"] = route.instant(item["createdAt"])
		if (RecordValues{}).Truthy(item["dueAt"]) {
			item["dueAt"] = route.instant(item["dueAt"])
		}
		listed = append(listed, item)
	}
	return Answer{}.Ok(listed)
}

// Create answers POST /reminders: a reminder on one of the caller's sent
// messages.
func (route RemindersRoute) Create(request *http.Request) *http.Response {
	caller, body, refusal := (DeliveryReply{}).Caller(route.Access, request)
	if refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.create(caller, body))
}

func (route RemindersRoute) create(caller RouteCaller, body map[string]any) (any, error) {
	emailId, _ := body["emailId"].(string)
	if emailId == "" {
		return map[string]any{"error": "emailId is required"}, nil
	}
	remindAtText, _ := body["remindAt"].(string)
	if remindAtText == "" {
		return map[string]any{"error": "remindAt is required"}, nil
	}
	remindAt := EpochTime{}.Parse(remindAtText)
	if math.IsNaN(remindAt) {
		return map[string]any{"error": "remindAt must be a valid ISO datetime"}, nil
	}
	if remindAt <= float64(EpochTime{}.Now()) {
		return map[string]any{"error": "remindAt must be in the future"}, nil
	}
	_, email, err := route.Emails.FindById(emailId)
	if err != nil {
		return nil, err
	}
	if email == nil || strings.ToLower(RecordValues{}.Text(email["sender"])) != caller.Email || !(TextValue{}).Includes(caller.Domains, email["domain"]) {
		return map[string]any{"error": "Sent email not found", "notFound": true}, nil
	}
	reminder, err := route.Reminders.Reminders().Schedule(map[string]any{
		"ownerEmail": caller.Email, "emailId": email["id"], "recipient": email["recipient"],
		"subject": email["subject"], "threadId": email["threadId"], "remindAt": int64(remindAt),
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": reminder["id"], "scheduled": true}, nil
}

// Dismiss answers DELETE /reminders.
func (route RemindersRoute) Dismiss(request *http.Request) *http.Response {
	caller, body, refusal := (DeliveryReply{}).Caller(route.Access, request)
	if refusal != nil {
		return refusal
	}
	id, _ := body["id"].(string)
	if id == "" {
		return Answer{}.BadRequest("id is required")
	}
	dismissed, err := route.Reminders.Reminders().Dismiss(id, caller.Email)
	if err != nil {
		return DeliveryReply{}.Settled(nil, err)
	}
	if !dismissed {
		return Answer{}.NotFound("Reminder not found")
	}
	return Answer{}.Ok(map[string]any{"dismissed": true})
}

// Tick answers POST /reminders/tick, signed with INBOUND_WEBHOOK_SECRET.
func (route RemindersRoute) Tick(request *http.Request) *http.Response {
	if _, refusal := (SystemSignature{}).Body(request, "INBOUND_WEBHOOK_SECRET"); refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.Reminders.Reminders().ProcessDueAll(EpochTime{}.Now()))
}

func (RemindersRoute) instant(value any) any {
	milliseconds, _ := DeliveryFields{}.Millis(value)
	return EpochTime{}.Text(milliseconds)
}
