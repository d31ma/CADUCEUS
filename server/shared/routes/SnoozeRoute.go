import (
	"errors"
	"math"
	"net/http"
	"regexp"
)

// SnoozeRoute answers /snooze (snooze a conversation, list active snoozes),
// /snooze/{id} (undo) and /snooze/tick (the signed due pass).
type SnoozeRoute struct {
	Access *AccessHelper
	Snooze *SnoozeService
}

// Create answers POST /snooze.
func (route SnoozeRoute) Create(request *http.Request) *http.Response {
	caller, body, refusal := (DeliveryReply{}).Caller(route.Access, request)
	if refusal != nil {
		return refusal
	}
	return route.settled(route.create(caller, body))
}

func (route SnoozeRoute) create(caller RouteCaller, body map[string]any) (any, error) {
	for _, required := range [][2]string{{"emailId", "emailId is required"}, {"until", "until (ISO datetime) is required"}} {
		if text, _ := body[required[0]].(string); text == "" {
			return map[string]any{"error": required[1]}, nil
		}
	}
	emailId, untilText := body["emailId"].(string), body["until"].(string)
	if _, isText := body["label"].(string); body["label"] != nil && !isText {
		return map[string]any{"error": "label must be text"}, nil
	}
	clientRequestId, isText := body["clientRequestId"].(string)
	if body["clientRequestId"] != nil && (!isText || !regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`).MatchString(clientRequestId)) {
		return map[string]any{"error": "clientRequestId is invalid"}, nil
	}
	until := EpochTime{}.Parse(untilText)
	if math.IsNaN(until) || math.IsInf(until, 0) {
		return map[string]any{"error": "until must be a valid ISO datetime"}, nil
	}
	input := map[string]any{"emailId": emailId, "until": int64(until), "label": body["label"], "clientRequestId": clientRequestId, "ownerEmail": caller.Email}
	return route.Snooze.Snoozes().SnoozeThread(input, caller.Domains)
}

// List answers GET /snooze.
func (route SnoozeRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.Snooze.Snoozes().ListActive(caller.Email, caller.Domains))
}

// Undo answers DELETE /snooze/{id}.
func (route SnoozeRoute) Undo(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if id == "" {
		return Answer{}.NotFound("Snooze not found")
	}
	return route.settled(route.Snooze.Snoozes().Undo(id, caller.Email))
}

// Tick answers POST /snooze/tick, signed with INBOUND_WEBHOOK_SECRET.
func (route SnoozeRoute) Tick(request *http.Request) *http.Response {
	if _, refusal := (SystemSignature{}).Body(request, "INBOUND_WEBHOOK_SECRET"); refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.Snooze.Snoozes().ProcessDue(EpochTime{}.Now()))
}

// settled answers a snooze refusal with its status.
func (SnoozeRoute) settled(answer any, err error) *http.Response {
	var refusal *DeliveryRefusal
	if errors.As(err, &refusal) {
		return DeliveryReply{}.Of(refusal.Answer())
	}
	return DeliveryReply{}.Settled(answer, err)
}
