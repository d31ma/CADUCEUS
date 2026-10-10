import "math"

// TrackingEvents builds the patches that record one open or click on a
// stored Sent copy: the event joins its JSON list (keeping the newest
// MaximumEventsPerMessage) and the message's counters move on.
type TrackingEvents struct{}

// Opened answers the patch of one open by recipient at openedAt, or nil when
// the message does not track opens.
func (events TrackingEvents) Opened(email map[string]any, recipient string, openedAt string) map[string]any {
	if email["openTracking"] != true {
		return nil
	}
	records := TrackingRecords{}
	opens := append(records.ReadArray(email["opensJson"]), map[string]any{
		"id": "open-" + Security{}.RandomUuid(), "recipient": records.NormalizeEmail(recipient), "openedAt": openedAt,
	})
	return map[string]any{
		"opensJson": StoredRecords{}.JsonText(events.latest(opens)),
		"openedAt":  RecordValues{}.Or(email["openedAt"], openedAt),
		"openCount": math.Max(0, RecordValues{}.NumberOf(email["openCount"])) + 1,
	}
}

// Clicked answers the patch of one click at clickedAt, or nil when the
// message does not track links or the click's target is not exactly the
// stored link with its index.
func (events TrackingEvents) Clicked(email map[string]any, click TrackingClick, clickedAt string) map[string]any {
	if email["linkTracking"] != true {
		return nil
	}
	link := events.link(email, click.LinkIndex)
	if link == nil || link["url"] != click.Url {
		return nil
	}
	records := TrackingRecords{}
	clicks := append(records.ReadArray(email["clicksJson"]), map[string]any{
		"id": "click-" + Security{}.RandomUuid(), "recipient": records.NormalizeEmail(click.Recipient),
		"url": link["url"], "title": records.CleanText(link["title"], 200),
		"linkIndex": click.LinkIndex, "clickedAt": clickedAt,
	})
	return map[string]any{
		"clicksJson": StoredRecords{}.JsonText(events.latest(clicks)),
		"clickCount": math.Max(0, RecordValues{}.NumberOf(email["clickCount"])) + 1,
	}
}

// link answers the stored link with the index, or nil.
func (TrackingEvents) link(email map[string]any, index float64) map[string]any {
	for _, value := range (TrackingRecords{}).ReadArray(email["trackedLinksJson"]) {
		link, _ := RecordValues{}.Object(value)
		if position, isNumber := (RecordValues{}).Number(link["index"]); isNumber && position == index {
			return link
		}
	}
	return nil
}

// latest keeps the newest MaximumEventsPerMessage events.
func (TrackingEvents) latest(events []any) []any {
	maximum := TrackingRecords{}.MaximumEventsPerMessage()
	if len(events) > maximum {
		return events[len(events)-maximum:]
	}
	return events
}
