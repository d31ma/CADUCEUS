import (
	"errors"
	"time"
)

// ProviderCalendars reads a caller's authenticated CalDAV resources into the
// ordinary calendar event model and writes event changes back as resource
// mutations, which the Go owner applies to the provider.
type ProviderCalendars struct {
	Accounts ProviderAccounts
}

// Available reports whether provider calendars can be reached: they come
// through the Go boundary, so without its address there are none.
func (ProviderCalendars) Available() bool {
	return Settings{}.Value("FYLO_BRIDGE_ADDRESS") != ""
}

// List answers the caller's provider events.
func (calendars ProviderCalendars) List(token string) ([]map[string]any, error) {
	_, events, _, err := calendars.load(token)
	return events, err
}

// Calendars answers the caller's provider calendars.
func (calendars ProviderCalendars) Calendars(token string) ([]any, error) {
	listed, _, _, err := calendars.load(token)
	return listed, err
}

// Find answers a provider event and its resource, or nil.
func (calendars ProviderCalendars) Find(token string, eventId string) (*ProviderEvent, error) {
	_, events, resources, err := calendars.load(token)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		if event["id"] == eventId {
			resource, known := resources[RecordValues{}.String(event["providerResourceId"])]
			if !known {
				return nil, nil
			}
			return &ProviderEvent{Event: event, Resource: resource}, nil
		}
	}
	return nil, nil
}

// Create writes a new event to one of the caller's writable calendars.
func (calendars ProviderCalendars) Create(token string, calendarId string, event map[string]any, idempotencyKey string) (map[string]any, error) {
	listed, _, _, err := calendars.load(token)
	if err != nil {
		return nil, err
	}
	var calendar map[string]any
	for _, candidate := range listed {
		entry, _ := RecordValues{}.Object(candidate)
		if entry != nil && entry["calendarId"] == calendarId && entry["writable"] != false {
			calendar = entry
			break
		}
	}
	if calendar == nil {
		return nil, errors.New("provider calendar not found")
	}
	resource, err := Ics{}.Resource(event, Security{}.RandomUuid()+"@caduceus.local")
	if err != nil {
		return nil, err
	}
	return calendars.mutate(token, calendar["accountId"], map[string]any{
		"idempotencyKey": idempotencyKey, "kind": "create", "calendarId": calendarId, "resourceId": "", "expectedEtag": "", "rawICalendar": resource,
	})
}

// Update writes an edited event, or one edited occurrence when
// recurrenceId names it, back to its resource.
func (calendars ProviderCalendars) Update(token string, change ProviderChange) (map[string]any, error) {
	found, err := calendars.Find(token, change.EventId)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, errors.New("provider calendar event not found")
	}
	desired := StoredRecords{}.Without(found.Event)
	for key, value := range change.Event {
		desired[key] = value
	}
	desired["recurrenceId"] = change.RecurrenceId
	desired["status"] = RecordValues{}.Or(change.Event["status"], "confirmed")
	if change.RecurrenceId != "" {
		desired["recurrence"] = ""
		desired["recurrenceDates"] = []any{}
		desired["exceptionDates"] = []any{}
	}
	resource, err := Ics{}.Updated(RecordValues{}.Text(found.Resource["rawICalendar"]), RecordValues{}.Text(found.Event["icsUid"]), change.RecurrenceId, desired)
	if err != nil {
		return nil, err
	}
	return calendars.mutate(token, found.Event["accountId"], ProviderAccounts{}.Defined(map[string]any{
		"idempotencyKey": change.IdempotencyKey, "kind": "update", "calendarId": "", "resourceId": found.Resource["id"], "expectedEtag": found.Resource["etag"], "rawICalendar": resource,
	}))
}

// Delete removes an event, or excludes one occurrence when recurrenceId
// names it.
func (calendars ProviderCalendars) Delete(token string, eventId string, recurrenceId string, idempotencyKey string) (map[string]any, error) {
	found, err := calendars.Find(token, eventId)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, errors.New("provider calendar event not found")
	}
	resource, deleteAll, err := Ics{}.Deleted(RecordValues{}.Text(found.Resource["rawICalendar"]), RecordValues{}.Text(found.Event["icsUid"]), recurrenceId)
	if err != nil {
		return nil, err
	}
	kind := "update"
	if deleteAll {
		kind = "delete"
	}
	return calendars.mutate(token, found.Event["accountId"], ProviderAccounts{}.Defined(map[string]any{
		"idempotencyKey": idempotencyKey, "kind": kind, "calendarId": "", "resourceId": found.Resource["id"], "expectedEtag": found.Resource["etag"], "rawICalendar": resource,
	}))
}

// load answers the caller's calendars, their events and the resources the
// events came from (by resource id).
func (calendars ProviderCalendars) load(token string) ([]any, []map[string]any, map[string]map[string]any, error) {
	response, err := calendars.Accounts.Object("accounts.calendar_resources", map[string]any{"token": token})
	if err != nil {
		return nil, nil, nil, err
	}
	values := RecordValues{}
	resources, _ := values.List(response["resources"])
	listed := []any{}
	rawCalendars, _ := values.List(response["calendars"])
	for _, candidate := range rawCalendars {
		if entry, _ := values.Object(candidate); entry != nil && values.Truthy(entry["calendarId"]) && values.Truthy(entry["accountId"]) {
			listed = append(listed, entry)
		}
	}
	rules := ProviderCalendarRules{}
	if len(resources) > rules.MostResources() {
		return nil, nil, nil, errors.New("provider calendar resource limit exceeded")
	}
	events := []map[string]any{}
	byId := map[string]map[string]any{}
	for _, candidate := range resources {
		resource, _ := values.Object(candidate)
		raw := values.Text(resource["rawICalendar"])
		if raw == "" || len(raw) > rules.LargestResource() {
			continue
		}
		calendar, isCalendar, err := Ics{}.Parse(raw)
		if err != nil {
			return nil, nil, nil, err
		}
		if !isCalendar {
			continue
		}
		byId[values.String(resource["id"])] = resource
		for _, event := range calendar.Events {
			if len(events) >= rules.MostEvents() {
				return nil, nil, nil, errors.New("provider calendar event limit exceeded")
			}
			events = append(events, calendars.event(resource, event))
		}
	}
	return listed, events, byId, nil
}

// event answers one provider VEVENT in the calendar event model.
func (calendars ProviderCalendars) event(resource map[string]any, event map[string]any) map[string]any {
	values := RecordValues{}
	rules := ProviderCalendarRules{}
	masterId := rules.EventId(values.String(resource["id"]), values.String(event["uid"]))
	recurrenceId := values.Text(values.Or(event["recurrenceId"], ""))
	id, seriesId := masterId, ""
	if recurrenceId != "" {
		id, seriesId = masterId+":exception:"+rules.ShortHash(recurrenceId), masterId
	}
	list := func(field string) []any {
		items, isList := values.List(event[field])
		if !isList {
			return []any{}
		}
		return items
	}
	return ProviderAccounts{}.Defined(map[string]any{
		"id":                 id,
		"userEmail":          resource["mailboxAddress"],
		"title":              event["summary"],
		"start":              event["start"],
		"end":                event["end"],
		"allDay":             values.Truthy(event["allDay"]),
		"location":           values.Or(event["location"], ""),
		"description":        values.Or(event["description"], ""),
		"attendees":          list("attendees"),
		"organizer":          values.Or(event["organizer"], resource["mailboxAddress"]),
		"source":             "provider",
		"icsUid":             event["uid"],
		"recurrence":         values.Or(event["rrule"], ""),
		"recurrenceDates":    list("recurrenceDates"),
		"exceptionDates":     list("exceptionDates"),
		"timeZone":           values.Or(event["timeZone"], "UTC"),
		"seriesId":           seriesId,
		"recurrenceId":       recurrenceId,
		"status":             values.Or(event["status"], "confirmed"),
		"readOnly":           false,
		"accountId":          resource["accountId"],
		"accountLabel":       values.Or(resource["accountLabel"], resource["mailboxAddress"]),
		"accountColor":       values.Or(resource["accountColor"], ""),
		"calendarId":         resource["calendarId"],
		"calendarName":       values.Or(resource["calendarName"], "Calendar"),
		"calendarColor":      values.Or(values.Or(resource["calendarColor"], resource["accountColor"]), ""),
		"providerResourceId": resource["id"],
		"createdAt":          resource["updatedAt"],
		"updatedAt":          resource["updatedAt"],
	})
}

// mutate sends one calendar mutation and waits for the owner to apply it.
func (calendars ProviderCalendars) mutate(token string, accountId any, input map[string]any) (map[string]any, error) {
	parameters := StoredRecords{}.Without(input)
	parameters["token"] = token
	if accountId != nil {
		parameters["id"] = accountId
	}
	mutation, err := calendars.Accounts.Object("accounts.calendar_mutate", parameters)
	if err != nil {
		return nil, err
	}
	if !(RecordValues{}).Truthy(mutation["id"]) {
		return mutation, nil
	}
	current := mutation
	for attempt := 0; attempt < 40 && (current["state"] == "queued" || current["state"] == "applying"); attempt++ {
		time.Sleep(time.Duration(50+min(attempt*25, 200)) * time.Millisecond)
		current, err = calendars.Accounts.Object("accounts.calendar_mutation", ProviderAccounts{}.Defined(map[string]any{"token": token, "id": accountId, "mutationId": mutation["id"]}))
		if err != nil {
			return nil, err
		}
	}
	if current["state"] == "conflict" {
		return nil, errors.New("the provider calendar changed; refresh and retry")
	}
	if current["state"] != "applied" {
		return nil, errors.New("provider calendar mutation did not complete")
	}
	return current, nil
}
