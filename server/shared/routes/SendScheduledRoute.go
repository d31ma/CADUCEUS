import (
	"net/http"
	"sort"
)

// SendScheduledRoute answers /send/scheduled and /send/scheduled/{id}: the
// caller's scheduled messages, which only their exact owner may see, edit
// or cancel.
type SendScheduledRoute struct {
	Access    *AccessHelper
	Send      *SendService
	Scheduled *ScheduledHelper
}

// List answers GET /send/scheduled, soonest first. Connected-account
// schedules the provider already sent or cancelled are dropped.
func (route SendScheduledRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.list(caller))
}

func (route SendScheduledRoute) list(caller RouteCaller) (any, error) {
	schedules := route.Send.Schedules()
	records, err := route.Scheduled.ListForOwner(caller.Email)
	if err != nil {
		return nil, err
	}
	listed := []map[string]any{}
	for _, stored := range records {
		record, err := schedules.FindOwned(RecordValues{}.Text(stored.Data["id"]), caller.Email)
		if err != nil {
			return nil, err
		}
		if record == nil {
			continue
		}
		var submission map[string]any
		if record.Entry["transport"] == "external" {
			if submission, err = schedules.Submission(record, caller.Token); err != nil {
				return ExternalAccounts{}.Refusal(err), nil
			}
			if submission["state"] == "sent" || submission["state"] == "cancelled" {
				if err := schedules.Sent.Scheduled.Delete(record.DocumentId); err != nil {
					return nil, err
				}
				continue
			}
		}
		presented, err := schedules.Present(record, false, submission)
		if err != nil {
			return nil, err
		}
		listed = append(listed, presented)
	}
	sort.SliceStable(listed, func(left, right int) bool {
		return (TextOrder{}).Compare(RecordValues{}.Text(listed[left]["sendAt"]), RecordValues{}.Text(listed[right]["sendAt"])) < 0
	})
	return listed, nil
}

// CancelNamed answers DELETE /send/scheduled, naming the message in the
// body.
func (route SendScheduledRoute) CancelNamed(request *http.Request) *http.Response {
	caller, body, refusal := (DeliveryReply{}).Caller(route.Access, request)
	if refusal != nil {
		return refusal
	}
	id, _ := body["id"].(string)
	if id == "" {
		return Answer{}.BadRequest("id is required")
	}
	return DeliveryReply{}.Settled(route.owned(caller, id, route.cancel))
}

// Cancel answers DELETE /send/scheduled/{id}.
func (route SendScheduledRoute) Cancel(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.owned(caller, id, route.cancel))
}

// Detail answers GET /send/scheduled/{id}, with attachment bytes for
// editing.
func (route SendScheduledRoute) Detail(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.owned(caller, id, route.detail))
}

// Edit answers PUT /send/scheduled/{id}.
func (route SendScheduledRoute) Edit(id string, request *http.Request) *http.Response {
	caller, body, refusal := (DeliveryReply{}).Caller(route.Access, request)
	if refusal != nil {
		return refusal
	}
	editing := route.Send.Editing()
	return DeliveryReply{}.Settled(route.owned(caller, id, func(caller RouteCaller, _ ScheduledSends, record *ScheduledRecord) (any, error) {
		return editing.Edit(record, caller, body)
	}))
}

// owned runs work on the caller's scheduled message with the id, or
// answers that there is none.
func (route SendScheduledRoute) owned(caller RouteCaller, id string, work func(RouteCaller, ScheduledSends, *ScheduledRecord) (any, error)) (any, error) {
	schedules := route.Send.Schedules()
	record, err := schedules.FindOwned(id, caller.Email)
	if err != nil || record == nil {
		return route.missing(), err
	}
	return work(caller, schedules, record)
}

func (route SendScheduledRoute) cancel(caller RouteCaller, schedules ScheduledSends, record *ScheduledRecord) (any, error) {
	var result map[string]any
	var err error
	if record.Entry["transport"] == "external" {
		result, err = schedules.CancelExternal(record, caller.Email, caller.Token)
	} else {
		result, err = schedules.Cancel(record, caller.Email)
	}
	if err != nil {
		return ExternalAccounts{}.Refusal(err), nil
	}
	if result["conflict"] == true {
		return map[string]any{"error": "Scheduled email is currently being delivered", "conflict": true}, nil
	}
	return result, nil
}

func (route SendScheduledRoute) detail(caller RouteCaller, schedules ScheduledSends, record *ScheduledRecord) (any, error) {
	var submission map[string]any
	var err error
	if record.Entry["transport"] == "external" {
		submission, err = schedules.Submission(record, caller.Token)
	}
	switch {
	case err != nil:
		return ExternalAccounts{}.Refusal(err), nil
	case submission["state"] == "sent" || submission["state"] == "cancelled":
		return route.missing(), schedules.Sent.Scheduled.Delete(record.DocumentId)
	}
	return schedules.Present(record, true, submission)
}

func (SendScheduledRoute) missing() map[string]any {
	return map[string]any{"error": "Scheduled email not found", "notFound": true}
}
