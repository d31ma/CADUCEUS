import "net/http"

// CalendarEventRoute answers /calendar/events/{id}: one event of the caller,
// local or from a provider calendar, and its edits and deletion, for the
// whole series or one occurrence.
type CalendarEventRoute struct {
	Access    *AccessHelper
	Events    *CalendarEventsHelper
	Providers *ProviderCalendarService
}

// Get answers one event.
func (route CalendarEventRoute) Get(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	_, event, _, refusal := route.located(caller, id)
	if refusal != nil {
		return refusal
	}
	shown := StoredRecords{}.Without(event)
	shown["readOnly"] = CalendarEventRules{}.ReadOnly(event, caller.Email) || (!(RecordValues{}).Truthy(event["recurrence"]) && (CalendarDate{}).IsPast(event, EpochTime{}.Now()))
	return Answer{}.Ok(EventView{}.Of(shown))
}

// Update applies the patchable fields of the body to the event, to the
// whole series or (scope=instance) to one occurrence.
func (route CalendarEventRoute) Update(id string, request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	edit := CalendarEdit{Caller: caller, Request: request, Body: body}
	edit.DocumentId, edit.Event, edit.Provider, refusal = route.located(caller, id)
	if refusal != nil {
		return refusal
	}
	if refusal := route.scoped(&edit); refusal != nil {
		return refusal
	}
	patch, refusal := route.patch(&edit)
	if refusal != nil {
		return refusal
	}
	if edit.Scope == "instance" {
		return route.updateOccurrence(edit, patch)
	}
	values := RecordValues{}
	next := StoredRecords{}.Without(edit.Event)
	for key, value := range patch {
		next[key] = value
	}
	route.keep(next, edit.Event, "id", "userEmail", "source", "createdAt")
	next["updatedAt"] = EpochTime{}.NowText()
	if edit.Provider {
		if _, err := route.Providers.Update(caller.Token, ProviderChange{EventId: values.String(edit.Event["id"]), Event: next, IdempotencyKey: (CalendarEventRules{}).IdempotencyKey(body["idempotencyKey"], "update")}); err != nil {
			return CalendarEventRules{}.ProviderFailure(err)
		}
	} else {
		if edit.DocumentId == "" {
			return Answer{}.NotFound("Event not found")
		}
		if err := route.replace(edit.DocumentId, next); err != nil {
			return RouteAccess{}.Failure(err)
		}
	}
	return Answer{}.Ok(map[string]any{"updated": true, "scope": route.seriesScope(edit.Event), "event": EventView{}.Of(next)})
}

// patch answers the validated fields of an edit, with the start and end it
// resolves to.
func (route CalendarEventRoute) patch(edit *CalendarEdit) (map[string]any, *http.Response) {
	values := RecordValues{}
	event := edit.Event
	patch := map[string]any{}
	for key, value := range edit.Body {
		if route.patchable(key) {
			patch[key] = value
		}
	}
	_, hasStart := patch["start"]
	_, hasEnd := patch["end"]
	nextStart, nextEnd := event["start"], event["end"]
	if text, isText := patch["start"].(string); isText {
		nextStart = text
	}
	if text, isText := patch["end"].(string); isText {
		nextEnd = text
	}
	allDay := values.Truthy(event["allDay"])
	if value, present := patch["allDay"]; present {
		flag, isFlag := value.(bool)
		if !isFlag {
			return nil, Answer{}.BadRequest("allDay must be a boolean")
		}
		allDay = flag
	}
	start, end, problem := CalendarDate{}.ValidateRange(nextStart, nextEnd, allDay)
	if problem != "" {
		return nil, Answer{}.BadRequest(problem)
	}
	if edit.Scope == "series" && values.Truthy(event["recurrence"]) && edit.OccurrenceStart != "" && (hasStart || hasEnd) {
		if !hasStart || !hasEnd {
			return nil, Answer{}.BadRequest("occurrence-relative series edits require both start and end")
		}
		_, occurrence, refusal := route.occurrence(event, edit.OccurrenceStart)
		if refusal != nil {
			return nil, refusal
		}
		if allDay != values.Truthy(event["allDay"]) {
			return nil, Answer{}.BadRequest("change the series all-day representation separately from an occurrence-relative resize")
		}
		start, end = CalendarEventRules{}.MasterRange(event, occurrence, start, end, allDay)
	}
	patch["start"], patch["end"] = start, end
	if title, isText := patch["title"].(string); isText {
		patch["title"] = Security{}.Trim(title)
		if patch["title"] == "" {
			return nil, Answer{}.BadRequest("title is required")
		}
	}
	zone := values.Text(values.Or(event["timeZone"], "UTC"))
	if text, isText := patch["timeZone"].(string); isText {
		zone = text
	}
	if !(CalendarZone{}).Valid(zone) {
		return nil, Answer{}.BadRequest("timeZone must be an IANA timezone")
	}
	if recurrence, present := patch["recurrence"]; present {
		if edit.Scope == "instance" {
			return nil, Answer{}.BadRequest("one occurrence cannot define a recurrence rule")
		}
		rule, problem := CalendarRecurrence{}.Validate(recurrence, start, zone)
		if problem != "" {
			return nil, Answer{}.BadRequest(problem)
		}
		patch["recurrence"] = rule
	}
	return patch, nil
}

// updateOccurrence stores an edited occurrence as an exception of its
// series (or writes it to the provider).
func (route CalendarEventRoute) updateOccurrence(edit CalendarEdit, patch map[string]any) *http.Response {
	event := edit.Event
	if !(RecordValues{}).Truthy(event["recurrence"]) {
		return Answer{}.BadRequest("instance scope requires a recurring series")
	}
	occurrenceStart, occurrence, refusal := route.occurrence(event, edit.OccurrenceStart)
	if refusal != nil {
		return refusal
	}
	existingDocumentId, existing := "", map[string]any(nil)
	if !edit.Provider {
		var err error
		if existingDocumentId, existing, err = route.Events.FindException(RecordValues{}.String(event["id"]), occurrenceStart); err != nil {
			return RouteAccess{}.Failure(err)
		}
	}
	exception := route.exception(event, existing, occurrence, patch, occurrenceStart, "confirmed")
	route.keep(exception, event, "source")
	if edit.Provider {
		change := ProviderChange{EventId: RecordValues{}.String(event["id"]), Event: exception, RecurrenceId: occurrenceStart, IdempotencyKey: CalendarEventRules{}.IdempotencyKey(edit.Body["idempotencyKey"], "update-instance")}
		if _, err := route.Providers.Update(edit.Caller.Token, change); err != nil {
			return CalendarEventRules{}.ProviderFailure(err)
		}
	} else if err := route.replace(existingDocumentId, exception); err != nil {
		return RouteAccess{}.Failure(err)
	}
	shown := StoredRecords{}.Without(exception)
	shown["isRecurring"], shown["isException"] = true, true
	return Answer{}.Ok(map[string]any{"updated": true, "scope": "instance", "event": EventView{}.Of(shown)})
}

// Delete deletes the event: the whole series, or (scope=instance) one
// occurrence, which is kept as a cancelled exception.
func (route CalendarEventRoute) Delete(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	edit := CalendarEdit{Caller: caller, Request: request}
	edit.DocumentId, edit.Event, edit.Provider, refusal = route.located(caller, id)
	if refusal != nil {
		return refusal
	}
	if refusal := route.scoped(&edit); refusal != nil {
		return refusal
	}
	if edit.Provider {
		return route.deleteFromProvider(edit)
	}
	event := edit.Event
	if edit.Scope == "instance" {
		return route.cancelOccurrence(edit)
	}
	if (RecordValues{}).Truthy(event["recurrence"]) {
		if _, err := route.Events.DeleteSeries(RecordValues{}.String(event["id"]), caller.Email); err != nil {
			return RouteAccess{}.Failure(err)
		}
	} else if edit.DocumentId != "" {
		if err := route.Events.Delete(edit.DocumentId); err != nil {
			return RouteAccess{}.Failure(err)
		}
	} else {
		return Answer{}.NotFound("Event not found")
	}
	return Answer{}.Ok(map[string]any{"deleted": true, "scope": route.seriesScope(event), "remainingSeries": false})
}

func (route CalendarEventRoute) deleteFromProvider(edit CalendarEdit) *http.Response {
	event := edit.Event
	recurrenceId := ""
	if edit.Scope == "instance" {
		recurrenceId = CalendarDate{}.NormalizeOccurrence(edit.OccurrenceStart, (RecordValues{}).Truthy(event["allDay"]))
		if !(RecordValues{}).Truthy(event["recurrence"]) || recurrenceId == "" || (CalendarEventRules{}).Occurrence(event, recurrenceId) == nil {
			return Answer{}.BadRequest("instance scope requires a valid recurring occurrence")
		}
	}
	key := CalendarEventRules{}.IdempotencyKey(RouteQuery{}.Value(edit.Request, "idempotencyKey"), "delete")
	if _, err := route.Providers.Delete(edit.Caller.Token, (RecordValues{}).String(event["id"]), recurrenceId, key); err != nil {
		return CalendarEventRules{}.ProviderFailure(err)
	}
	scope := route.seriesScope(event)
	if recurrenceId != "" {
		scope = "instance"
	}
	return Answer{}.Ok(map[string]any{"deleted": true, "scope": scope, "remainingSeries": recurrenceId != ""})
}

func (route CalendarEventRoute) cancelOccurrence(edit CalendarEdit) *http.Response {
	event := edit.Event
	if !(RecordValues{}).Truthy(event["recurrence"]) {
		return Answer{}.BadRequest("instance scope requires a recurring series")
	}
	occurrenceStart, occurrence, refusal := route.occurrence(event, edit.OccurrenceStart)
	if refusal != nil {
		return refusal
	}
	existingDocumentId, existing, err := route.Events.FindException(RecordValues{}.String(event["id"]), occurrenceStart)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if err := route.replace(existingDocumentId, route.exception(event, existing, occurrence, nil, occurrenceStart, "cancelled")); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"deleted": true, "scope": "instance", "remainingSeries": true})
}

// located answers the event with an id: the caller's local event, or one of
// their provider events (provider is true, with no document id).
func (route CalendarEventRoute) located(caller RouteCaller, id string) (string, map[string]any, bool, *http.Response) {
	if id == "" {
		return "", nil, false, Answer{}.NotFound("Event not found")
	}
	documentId, event, err := route.Events.FindById(id)
	if err != nil {
		return "", nil, false, RouteAccess{}.Failure(err)
	}
	provider := false
	if (event == nil || documentId == "") && route.Providers.Available() {
		found, err := route.Providers.Find(caller.Token, id)
		if err != nil {
			return "", nil, false, ExternalAccounts{}.Failure(err)
		}
		if found != nil {
			event, documentId, provider = found.Event, "", true
		}
	}
	if event == nil || (documentId == "" && !provider) {
		return "", nil, false, Answer{}.NotFound("Event not found")
	}
	if !provider && event["userEmail"] != caller.Email {
		return "", nil, false, Answer{}.Forbidden("Access denied")
	}
	return documentId, event, provider, nil
}

// scoped reads the edit's scope and refuses edits the caller may not make.
func (route CalendarEventRoute) scoped(edit *CalendarEdit) *http.Response {
	edit.Scope = RouteQuery{}.Value(edit.Request, "scope")
	if edit.Scope == "" {
		edit.Scope = "series"
	}
	if edit.Scope != "series" && edit.Scope != "instance" {
		return Answer{}.BadRequest("scope must be series or instance")
	}
	edit.OccurrenceStart = RouteQuery{}.Value(edit.Request, "occurrenceStart")
	return CalendarEventRules{}.MutationRefusal(edit.Event, edit.Caller.Email, edit.OccurrenceStart)
}

// occurrence answers the identity and generated occurrence a series edit
// names.
func (route CalendarEventRoute) occurrence(event map[string]any, occurrenceStart string) (string, map[string]any, *http.Response) {
	normalized := CalendarDate{}.NormalizeOccurrence(occurrenceStart, (RecordValues{}).Truthy(event["allDay"]))
	if normalized == "" {
		return "", nil, Answer{}.BadRequest("occurrenceStart must identify a valid series occurrence")
	}
	occurrence := CalendarEventRules{}.Occurrence(event, normalized)
	if occurrence == nil {
		return "", nil, Answer{}.NotFound("Occurrence not found")
	}
	return normalized, occurrence, nil
}

// exception answers the stored exception of one occurrence: the existing
// exception (or the generated occurrence) with the patch over it.
func (route CalendarEventRoute) exception(event map[string]any, existing map[string]any, occurrence map[string]any, patch map[string]any, recurrenceId string, status string) map[string]any {
	values := RecordValues{}
	base := occurrence
	if existing != nil {
		base = existing
	}
	exception := StoredRecords{}.Without(base)
	for key, value := range patch {
		exception[key] = value
	}
	now := EpochTime{}.NowText()
	exception["id"] = values.Or(existing["id"], CalendarEventRules{}.NewId())
	route.keep(exception, event, "userEmail")
	exception["seriesId"] = event["id"]
	exception["recurrenceId"] = recurrenceId
	exception["recurrence"] = ""
	exception["recurrenceDates"] = []any{}
	exception["exceptionDates"] = []any{}
	exception["status"] = status
	exception["createdAt"] = values.Or(existing["createdAt"], now)
	exception["updatedAt"] = now
	return exception
}

// replace deletes the prior document (FYLO's put always inserts under a new
// document id) and stores the event.
func (route CalendarEventRoute) replace(documentId string, event map[string]any) error {
	if documentId != "" {
		if err := route.Events.Delete(documentId); err != nil {
			return err
		}
	}
	_, err := route.Events.Put(event)
	return err
}

// keep copies fields from source, leaving out the ones source does not have.
func (CalendarEventRoute) keep(target map[string]any, source map[string]any, fields ...string) {
	for _, field := range fields {
		if value, present := source[field]; present {
			target[field] = value
		} else {
			delete(target, field)
		}
	}
}

func (CalendarEventRoute) seriesScope(event map[string]any) string {
	if (RecordValues{}).Truthy(event["recurrence"]) {
		return "series"
	}
	return "event"
}

func (CalendarEventRoute) patchable(key string) bool {
	for _, candidate := range []string{"title", "start", "end", "allDay", "location", "description", "attendees", "organizer", "recurrence", "reminderMinutes", "status", "timeZone"} {
		if key == candidate {
			return true
		}
	}
	return false
}
