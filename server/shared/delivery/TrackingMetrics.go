import (
	"math"
	"sort"
)

// TrackingMetrics summarizes tracked mail by subject and by UTC day.
type TrackingMetrics struct{}

// BySubject answers the 20 subjects sent most, with how many of each were
// opened, clicked and replied to.
func (metrics TrackingMetrics) BySubject(sent []map[string]any, repliesByThread map[string]bool) []map[string]any {
	records := TrackingRecords{}
	groups := []map[string]any{}
	index := map[string]int{}
	for _, email := range sent {
		subject := records.CleanText(email["subject"], 500)
		if subject == "" {
			subject = "(No subject)"
		}
		position, known := index[subject]
		if !known {
			position = len(groups)
			index[subject] = position
			groups = append(groups, map[string]any{"subject": subject, "sent": 0, "opened": 0, "clicked": 0, "replied": 0})
		}
		item := groups[position]
		metrics.count(item, "sent", true)
		metrics.count(item, "opened", records.HasOpen(email))
		metrics.count(item, "clicked", records.HasClick(email))
		metrics.count(item, "replied", records.Replied(email, repliesByThread))
	}
	sort.SliceStable(groups, func(left, right int) bool {
		leftSent, rightSent := groups[left]["sent"].(int), groups[right]["sent"].(int)
		if leftSent != rightSent {
			return leftSent > rightSent
		}
		return TextOrder{}.Compare(groups[left]["subject"].(string), groups[right]["subject"].(string)) < 0
	})
	if len(groups) > metrics.maximumSubjects() {
		groups = groups[:metrics.maximumSubjects()]
	}
	return groups
}

func (TrackingMetrics) count(item map[string]any, field string, counted bool) {
	if counted {
		item[field] = item[field].(int) + 1
	}
}

func (TrackingMetrics) maximumSubjects() int { return 20 }

// Daily answers, for each UTC day from the start of the window (at most 90
// days back, 30 when the window has no start) to now, how much was sent,
// received, opened and clicked.
func (metrics TrackingMetrics) Daily(sent []map[string]any, received []map[string]any, events []map[string]any, from float64, now float64) []map[string]any {
	records := TrackingRecords{}
	day := records.DayMilliseconds()
	first := now - metrics.defaultDays()*day
	if !math.IsInf(from, 0) && !math.IsNaN(from) && from > 0 {
		first = from
	}
	first = math.Max(first, now-metrics.maximumDays()*day)
	days := []map[string]any{}
	index := map[string]int{}
	for at := records.StartOfDay(first); at <= records.StartOfDay(now); at += day {
		date := EpochTime{}.Text(int64(at))[:len("2006-01-02")]
		index[date] = len(days)
		days = append(days, map[string]any{"date": date, "sent": 0, "received": 0, "opens": 0, "clicks": 0})
	}
	for _, email := range sent {
		metrics.increment(days, index, email["receivedAt"], "sent")
	}
	for _, email := range received {
		metrics.increment(days, index, email["receivedAt"], "received")
	}
	for _, event := range events {
		field := "clicks"
		if event["type"] == "open" {
			field = "opens"
		}
		metrics.increment(days, index, event["timestamp"], field)
	}
	return days
}

func (TrackingMetrics) increment(days []map[string]any, index map[string]int, value any, field string) {
	timestamp := TrackingRecords{}.CleanTimestamp(value)
	if timestamp == "" {
		return
	}
	if position, known := index[timestamp[:len("2006-01-02")]]; known {
		days[position][field] = days[position][field].(int) + 1
	}
}

func (TrackingMetrics) defaultDays() float64 { return 29 }

func (TrackingMetrics) maximumDays() float64 { return 89 }
