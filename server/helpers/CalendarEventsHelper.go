import "math"

// CalendarEventsHelper keeps calendar events, each owned by one user.
// @Helper
type CalendarEventsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores an event and answers its document id.
func (helper *CalendarEventsHelper) Put(event map[string]any) (string, error) {
	return helper.store().Put(CalendarEventsRecords{}.Stored(event))
}

// List answers every event of a user as {docId, ...event}. With a start or
// end (ISO text; "" for none) it keeps the events that overlap that window,
// and every recurring master and exception, which the recurrence expansion
// needs whatever their own dates.
func (helper *CalendarEventsHelper) List(userEmail string, start string, end string) ([]map[string]any, error) {
	documents, err := helper.store().Equal(helper.userField(), userEmail)
	if err != nil {
		return nil, err
	}
	events := helper.hydrated(documents)
	if start == "" && end == "" {
		return events, nil
	}
	windowStart, windowEnd := math.Inf(-1), math.Inf(1)
	if start != "" {
		windowStart = EpochTime{}.Parse(start)
	}
	if end != "" {
		windowEnd = EpochTime{}.Parse(end)
	}
	kept := []map[string]any{}
	for _, event := range events {
		recurring := RecordValues{}.Truthy(event["recurrence"]) || RecordValues{}.Truthy(event[helper.seriesField()])
		if recurring || (CalendarDate{}).Overlaps(event, windowStart, windowEnd) {
			kept = append(kept, event)
		}
	}
	return kept, nil
}

// FindById answers the document id and event with the id, or "" and nil.
func (helper *CalendarEventsHelper) FindById(id string) (string, map[string]any, error) {
	documents, err := helper.store().Equal(helper.idField(), id)
	if err != nil || len(documents) == 0 {
		return "", nil, err
	}
	return documents[0].Id, helper.hydrated(documents[:1])[0], nil
}

// FindByIcsUid answers the user's event (not an exception) with an ICS UID,
// or "" and nil. FYLO joins conditions with OR, so the query narrows by user
// and the UID is checked here.
func (helper *CalendarEventsHelper) FindByIcsUid(userEmail string, icsUid string) (string, map[string]any, error) {
	documents, err := helper.store().Equal(helper.userField(), userEmail)
	for _, document := range documents {
		raw := StoredRecords{}.Decoded(document.Data)
		if raw["icsUid"] == icsUid && !(RecordValues{}).Truthy(raw[helper.recurrenceField()]) {
			return document.Id, StoredRecords{}.WithDocumentId(document.Id, CalendarEventsRecords{}.Hydrated(raw)), nil
		}
	}
	return "", nil, err
}

// FindByIdempotencyKey answers the user's event created with an idempotency
// key, or "" and nil.
func (helper *CalendarEventsHelper) FindByIdempotencyKey(userEmail string, idempotencyKey string) (string, map[string]any, error) {
	events, err := helper.List(userEmail, "", "")
	for _, event := range events {
		if event["idempotencyKey"] == idempotencyKey {
			return RecordValues{}.Text(event[helper.documentField()]), event, nil
		}
	}
	return "", nil, err
}

// FindException answers the exception of a series for one occurrence (a
// date, or an instant compared exactly), or "" and nil.
func (helper *CalendarEventsHelper) FindException(seriesId string, recurrenceId string) (string, map[string]any, error) {
	documents, err := helper.store().Equal(helper.seriesField(), seriesId)
	if err != nil {
		return "", nil, err
	}
	date, instant := helper.occurrence(recurrenceId)
	if date == "" && instant == "" {
		return "", nil, nil
	}
	for _, document := range documents {
		raw := StoredRecords{}.Decoded(document.Data)
		value := raw[helper.recurrenceField()]
		matchesDate := date != "" && (CalendarDate{}).Coerce(value) == date
		matchesInstant := instant != "" && (CalendarDate{}).IsDateTime(value) && helper.instant(value.(string)) == instant
		if matchesDate || matchesInstant {
			return document.Id, StoredRecords{}.WithDocumentId(document.Id, CalendarEventsRecords{}.Hydrated(raw)), nil
		}
	}
	return "", nil, nil
}

// DeleteSeries deletes a master and every stored exception of one owner and
// answers how many documents it deleted.
func (helper *CalendarEventsHelper) DeleteSeries(seriesId string, userEmail string) (int, error) {
	events, err := helper.List(userEmail, "", "")
	if err != nil {
		return 0, err
	}
	documentIds := []string{}
	for _, event := range events {
		documentId := RecordValues{}.Text(event[helper.documentField()])
		inSeries := event[helper.idField()] == seriesId || event[helper.seriesField()] == seriesId
		if inSeries && documentId != "" && !(PreferenceChoices{}).Includes(documentIds, documentId) {
			documentIds = append(documentIds, documentId)
		}
	}
	for _, documentId := range documentIds {
		if err := helper.Delete(documentId); err != nil {
			return 0, err
		}
	}
	return len(documentIds), nil
}

// Delete deletes one event document.
func (helper *CalendarEventsHelper) Delete(documentId string) error {
	return helper.store().Delete(documentId)
}

// occurrence answers a recurrence id as a calendar day or as an instant.
func (helper *CalendarEventsHelper) occurrence(recurrenceId string) (string, string) {
	if (CalendarDate{}).IsDate(recurrenceId) {
		return recurrenceId, ""
	}
	if (CalendarDate{}).IsDateTime(recurrenceId) {
		return "", helper.instant(recurrenceId)
	}
	return "", ""
}

func (CalendarEventsHelper) instant(text string) string {
	return EpochTime{}.Text(int64(EpochTime{}.Parse(text)))
}

func (CalendarEventsHelper) hydrated(documents []FyloDocument) []map[string]any {
	events := []map[string]any{}
	for _, document := range documents {
		event := CalendarEventsRecords{}.Hydrated(StoredRecords{}.Decoded(document.Data))
		events = append(events, StoredRecords{}.WithDocumentId(document.Id, event))
	}
	return events
}

func (CalendarEventsHelper) idField() string { return "id" }

func (CalendarEventsHelper) userField() string { return "userEmail" }

func (CalendarEventsHelper) seriesField() string { return "seriesId" }

func (CalendarEventsHelper) recurrenceField() string { return "recurrenceId" }

func (CalendarEventsHelper) documentField() string { return "docId" }

func (helper *CalendarEventsHelper) store() CollectionStore {
	return CollectionStore{Database: helper.fyloHelper.Database(), Collection: FyloCollections{}.Named().CalendarEvents}
}
