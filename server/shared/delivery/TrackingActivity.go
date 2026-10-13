import (
	"math"
	"sort"
	"strings"
)

// TrackingActivity builds the exact-owner activity feed and analytics of a
// mailbox from stored mail. Only Sent copies the mailbox owns, and replies
// addressed to the mailbox, count, even when several users share a domain.
type TrackingActivity struct {
	Options TrackingActivityOptions
	// Now is Date.now() when the options name no end.
	Now int64
}

// Built answers {events, unseenCount, metrics, generatedAt} for the mail.
func (activity TrackingActivity) Built(mail []map[string]any) map[string]any {
	records := TrackingRecords{}
	options := activity.Options
	now := activity.finite(options.To, float64(activity.Now))
	from := activity.finite(options.From, 0)
	owner := records.NormalizeEmail(options.OwnerEmail)
	senders := activity.senders(owner)
	sent, received := activity.split(mail, owner, senders)
	repliesByThread := records.ReplyThreads(received, sent, senders)
	events := activity.events(sent)
	matching := activity.matching(events, from, now)
	rangeSent := activity.within(sent, from, now)
	rangeReceived := activity.within(received, from, now)
	counts := activity.counts(rangeSent, repliesByThread)
	return map[string]any{
		"events":      matching[:min(len(matching), activity.limit())],
		"unseenCount": activity.unseen(matching),
		"metrics": map[string]any{
			"sent":         len(rangeSent),
			"received":     len(rangeReceived),
			"tracked":      counts["tracked"],
			"opened":       counts["opened"],
			"clicked":      counts["clicked"],
			"replied":      counts["replied"],
			"trackingRate": records.Rate(counts["tracked"], len(rangeSent)),
			"openRate":     records.Rate(counts["opened"], counts["openTracked"]),
			"clickRate":    records.Rate(counts["clicked"], counts["linkTracked"]),
			"replyRate":    records.Rate(counts["replied"], len(rangeSent)),
			"bySubject":    TrackingMetrics{}.BySubject(rangeSent, repliesByThread),
			"daily":        TrackingMetrics{}.Daily(rangeSent, rangeReceived, events, from, now),
		},
		"generatedAt": EpochTime{}.Text(int64(now)),
	}
}

func (TrackingActivity) finite(value *float64, fallback float64) float64 {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return fallback
	}
	return *value
}

func (activity TrackingActivity) limit() int {
	limit := math.Trunc(activity.finiteLimit())
	return int(math.Max(1, math.Min(TrackingRecords{}.MaximumActivityEvents(), limit)))
}

func (activity TrackingActivity) finiteLimit() float64 {
	if activity.Options.Limit == nil {
		return activity.defaultLimit()
	}
	return *activity.Options.Limit
}

func (TrackingActivity) defaultLimit() float64 { return 50 }

func (activity TrackingActivity) senders(owner string) map[string]bool {
	senders := map[string]bool{}
	for _, sender := range append([]string{owner}, activity.Options.OwnedSenders...) {
		if address := (TrackingRecords{}).NormalizeEmail(sender); address != "" {
			senders[address] = true
		}
	}
	return senders
}

// split answers the owner's newest Sent copies (at most
// MaximumActivityMessages) and the mail the owner received, within the
// allowed domains.
func (activity TrackingActivity) split(mail []map[string]any, owner string, senders map[string]bool) ([]map[string]any, []map[string]any) {
	records := TrackingRecords{}
	domains := map[string]bool{}
	for _, domain := range activity.Options.AllowedDomains {
		domains[strings.ToLower(domain)] = true
	}
	sent, received := []map[string]any{}, []map[string]any{}
	for _, email := range mail {
		if !domains[strings.ToLower(RecordValues{}.String(RecordValues{}.Or(email["domain"], "")))] {
			continue
		}
		if records.IsOwnedSent(email, owner, senders, activity.Options.SentFolders) {
			sent = append(sent, email)
		}
		if records.IsReceivedByOwner(email, senders, activity.Options.SentFolders, activity.Options.DraftFolders) {
			received = append(received, email)
		}
	}
	sort.SliceStable(sent, func(left, right int) bool {
		return records.TimestampOf(sent[left]["receivedAt"]) > records.TimestampOf(sent[right]["receivedAt"])
	})
	if len(sent) > records.MaximumActivityMessages() {
		sent = sent[:records.MaximumActivityMessages()]
	}
	return sent, received
}

// events answers every open and click of the sent mail.
func (activity TrackingActivity) events(sent []map[string]any) []map[string]any {
	records := TrackingRecords{}
	events := []map[string]any{}
	for _, email := range sent {
		fallback := ""
		if recipients := records.RecipientList(email); len(recipients) > 0 {
			fallback = recipients[0]
		}
		opens := records.ReadArray(email["opensJson"])
		for _, value := range opens {
			item, _ := RecordValues{}.Object(value)
			events = activity.appended(events, "open", email, item, item["openedAt"], fallback)
		}
		if (RecordValues{}).Truthy(email["openedAt"]) && len(opens) == 0 {
			events = activity.appended(events, "open", email, map[string]any{}, email["openedAt"], fallback)
		}
		for _, value := range records.ReadArray(RecordValues{}.Or(email["clicksJson"], email["clicks"])) {
			item, _ := RecordValues{}.Object(value)
			events = activity.appended(events, "click", email, item, RecordValues{}.Or(item["clickedAt"], item["timestamp"]), fallback)
		}
	}
	return events
}

// appended adds one event when its time is readable.
func (TrackingActivity) appended(events []map[string]any, kind string, email map[string]any, item map[string]any, at any, fallback string) []map[string]any {
	timestamp := TrackingRecords{}.CleanTimestamp(at)
	if timestamp == "" {
		return events
	}
	return append(events, TrackingRecords{}.ActivityEvent(kind, email, item, timestamp, fallback))
}

func (activity TrackingActivity) matching(events []map[string]any, from float64, now float64) []map[string]any {
	kind := activity.Options.Type
	matching := []map[string]any{}
	for _, event := range events {
		at := TrackingRecords{}.TimestampOf(event["timestamp"])
		typed := (kind != "open" && kind != "click") || event["type"] == kind
		if typed && at >= from && at <= now {
			matching = append(matching, event)
		}
	}
	sort.SliceStable(matching, func(left, right int) bool {
		return TextOrder{}.Compare(matching[right]["timestamp"].(string), matching[left]["timestamp"].(string)) < 0
	})
	return matching
}

func (activity TrackingActivity) unseen(matching []map[string]any) int {
	since := activity.Options.Since
	if since == nil || math.IsNaN(*since) || math.IsInf(*since, 0) {
		return 0
	}
	count := 0
	for _, event := range matching {
		if (TrackingRecords{}).TimestampOf(event["timestamp"]) > *since {
			count++
		}
	}
	return count
}

func (TrackingActivity) within(mail []map[string]any, from float64, now float64) []map[string]any {
	kept := []map[string]any{}
	for _, email := range mail {
		if (TrackingRecords{}).Within(email["receivedAt"], from, now) {
			kept = append(kept, email)
		}
	}
	return kept
}

// counts answers how many messages of the window were tracked, opened,
// clicked and replied to.
func (TrackingActivity) counts(sent []map[string]any, repliesByThread map[string]bool) map[string]int {
	records := TrackingRecords{}
	counts := map[string]int{}
	for _, email := range sent {
		openTracked, linkTracked := email["openTracking"] == true, email["linkTracking"] == true
		for field, counted := range map[string]bool{
			"opened": records.HasOpen(email), "clicked": records.HasClick(email),
			"replied": records.Replied(email, repliesByThread), "openTracked": openTracked,
			"linkTracked": linkTracked, "tracked": openTracked || linkTracked,
		} {
			if counted {
				counts[field]++
			}
		}
	}
	return counts
}
