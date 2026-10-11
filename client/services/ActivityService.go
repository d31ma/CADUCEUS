import (
	"cmp"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// @Service
type ActivityService struct {
	// @Needs
	activityClient            *ActivityClient
	applicationEventPublisher ApplicationEventPublisher
	Events                    []ActivityEventRow
	Metrics                   ActivityTotals
	Daily                     []ActivityDayRow
	// @SessionState
	Subjects         []ActivitySubjectRow
	Type             string
	Range            string
	Loaded           bool
	Error            string
	signedIn         bool
	EventFilterLabel string
	authorization    string
}

// @OnEvent({"replay":true})
func (activity *ActivityService) SessionChanged(session SessionChanged) {
	activity.signedIn = session.SignedIn
	activity.authorization = "Bearer " + session.Token
	activity.loadWhenSignedIn()
	activity.settle()
}

// Load reads the delivery insights for the chosen event type and range.
func (activity *ActivityService) Load() {
	activity.Type = cmp.Or(activity.Type, activity.everyKind())
	activity.Range = cmp.Or(activity.Range, activity.defaultRange())
	activity.Loaded = false
	activity.Error = ""
	report, failure := activity.activityClient.Report(activity.authorization, activity.Type, activity.Range, activity.limit())
	report = activity.reportUnless(report, failure)
	activity.Events = activity.eventRows(report.Events)
	activity.Metrics = activity.summary(report.Metrics)
	activity.Daily = activity.dayRows(report.Metrics.Daily)
	activity.Subjects = activity.subjectRows(report.Metrics.BySubject)
	activity.Error = activity.failureText(failure)
	activity.EventFilterLabel = cmp.Or(map[string]string{activity.openKind(): "open events", activity.clickKind(): "click events"}[activity.Type], "events")
	activity.markViewedWhenLoaded(report.GeneratedAt, failure)
	activity.Loaded = true
}

func (ActivityService) limit() int { return 100 }

func (ActivityService) everyKind() string { return "all" }

func (ActivityService) openKind() string { return "open" }

func (ActivityService) clickKind() string { return "click" }

func (ActivityService) defaultRange() string { return "30d" }

// Open opens the message an event belongs to.
func (activity *ActivityService) Open(emailID string) {
	activity.openWhenGiven(emailID)
}

// Settled renders the page after a listener that waited for the server.
//
// @OnEvent
func (activity *ActivityService) Settled(_ ViewSettled) {}

func (activity *ActivityService) openWhenGiven(emailID string) {
	if emailID != "" {
		activity.applicationEventPublisher.PublishEvent(ComposeUtils{}.Message(emailID), false)
	}
}

// SetType shows all, open or click events.
func (activity *ActivityService) SetType(kind string) {
	activity.loadWhenAllowed(&activity.Type, kind, []string{activity.everyKind(), activity.openKind(), activity.clickKind()})
}

// ChangeRange shows the last 7, 30 or 90 days, or all time.
func (activity *ActivityService) ChangeRange(period string) {
	activity.loadWhenAllowed(&activity.Range, period, []string{"7d", activity.defaultRange(), "90d", activity.everyKind()})
}

func (activity *ActivityService) loadWhenSignedIn() {
	if activity.signedIn {
		activity.Load()
	}
}

func (activity *ActivityService) loadWhenAllowed(field *string, value string, allowed []string) {
	if slices.Contains(allowed, value) {
		*field = value
		activity.Load()
	}
}

func (ActivityService) reportUnless(report ActivityReport, failure error) ActivityReport {
	if failure != nil {
		return ActivityReport{}
	}
	return report
}

func (ActivityService) failureText(failure error) string {
	if failure != nil {
		return "Activity could not be loaded."
	}
	return ""
}

func (activity *ActivityService) markViewedWhenLoaded(generatedAt string, failure error) {
	if failure == nil {
		activity.applicationEventPublisher.PublishEvent(ActivityViewed{At: generatedAt}, false)
	}
}

func (activity *ActivityService) eventRows(events []ActivityEvent) []ActivityEventRow {
	rows := []ActivityEventRow{}
	for _, event := range events {
		rows = append(rows, activity.eventRow(event))
	}
	return rows
}

func (activity *ActivityService) eventRow(event ActivityEvent) ActivityEventRow {
	kind := map[bool]string{true: activity.clickKind(), false: activity.openKind()}[event.Type == activity.clickKind()]
	return ActivityEventRow{
		ID: event.ID, Type: kind, MessageID: event.MessageID, Timestamp: event.Timestamp,
		EventIcon:      map[string]string{activity.openKind(): "◉", activity.clickKind(): "↗"}[kind],
		EventLabel:     map[string]string{activity.openKind(): "Opened", activity.clickKind(): "Clicked"}[kind],
		SubjectLabel:   cmp.Or(activity.clean(event.Subject), activity.noSubject()),
		RecipientLabel: cmp.Or(activity.clean(event.Recipient), "Recipient unavailable"),
		TargetLabel:    map[string]string{activity.clickKind(): cmp.Or(activity.clean(event.Title), activity.linkLabel(event.URL))}[kind],
		TimestampLabel: DateLabelUtils{}.Numeric(event.Timestamp),
	}
}

func (ActivityService) noSubject() string { return "(No subject)" }

func (activity *ActivityService) clean(text string) string {
	return strings.TrimSpace(strings.Map(activity.visible, text))
}

func (ActivityService) visible(character rune) rune {
	if unicode.IsControl(character) {
		return ' '
	}
	return character
}

func (ActivityService) linkLabel(raw string) string {
	target, failure := url.Parse(raw)
	if failure != nil || target.Scheme == "" || target.Host == "" {
		return "Tracked link"
	}
	separator := "/"
	rest := strings.TrimPrefix(target.EscapedPath(), separator)
	return target.Hostname() + map[bool]string{true: "", false: separator + rest}[rest == ""]
}

func (activity *ActivityService) summary(metrics ActivityMetrics) ActivityTotals {
	return ActivityTotals{
		Sent: metrics.Sent, Received: metrics.Received, Tracked: metrics.Tracked,
		Opened: metrics.Opened, Clicked: metrics.Clicked, Replied: metrics.Replied,
		TrackingRateLabel: activity.percent(metrics.TrackingRate), OpenRateLabel: activity.percent(metrics.OpenRate),
		ClickRateLabel: activity.percent(metrics.ClickRate), ReplyRateLabel: activity.percent(metrics.ReplyRate),
	}
}

func (activity *ActivityService) dayRows(days []ActivityDay) []ActivityDayRow {
	highest := activity.highest(days)
	rows := []ActivityDayRow{}
	for _, day := range days {
		label := DateLabelUtils{}.MonthDay(day.Date)
		rows = append(rows, ActivityDayRow{
			DateLabel: label,
			AriaLabel: fmt.Sprintf("%s: %s sent, %s received, %s opens, %s clicks", label,
				activity.count(day.Sent), activity.count(day.Received), activity.count(day.Opens), activity.count(day.Clicks)),
			SentStyle: activity.bar(day.Sent, highest), ReceivedStyle: activity.bar(day.Received, highest),
			OpensStyle: activity.bar(day.Opens, highest), ClicksStyle: activity.bar(day.Clicks, highest),
		})
	}
	return rows
}

func (activity *ActivityService) highest(days []ActivityDay) float64 {
	highest := 1.0
	for _, day := range days {
		highest = max(highest, activity.positive(day.Sent), activity.positive(day.Received), activity.positive(day.Opens), activity.positive(day.Clicks))
	}
	return highest
}

func (activity *ActivityService) bar(value float64, highest float64) string {
	number := activity.positive(value)
	height := map[bool]float64{true: 0, false: max(activity.shortestBar(), math.Round(number/highest*activity.hundred()))}[number == 0]
	return fmt.Sprintf("height:%v%%", height)
}

func (ActivityService) hundred() float64 { return 100 }

func (ActivityService) shortestBar() float64 { return 4 }

func (activity *ActivityService) subjectRows(subjects []ActivitySubject) []ActivitySubjectRow {
	rows := []ActivitySubjectRow{}
	for _, subject := range subjects {
		sent := activity.positive(subject.Sent)
		rows = append(rows, ActivitySubjectRow{
			SubjectLabel: cmp.Or(activity.clean(subject.Subject), activity.noSubject()), Sent: subject.Sent,
			OpenRateLabel:  activity.percent(activity.share(subject.Opened, sent)),
			ClickRateLabel: activity.percent(activity.share(subject.Clicked, sent)),
			ReplyRateLabel: activity.percent(activity.share(subject.Replied, sent)),
		})
	}
	return rows
}

func (activity *ActivityService) share(part float64, whole float64) float64 {
	return map[bool]float64{true: 0, false: activity.positive(part) / whole * activity.hundred()}[whole == 0]
}

func (ActivityService) positive(value float64) float64 {
	if value > 0 && !math.IsInf(value, 1) {
		return value
	}
	return 0
}

func (activity *ActivityService) percent(value float64) string {
	digits := map[bool]int{true: 1, false: 0}[math.Mod(value, 1) != 0]
	tenths := activity.tenths()
	text := strings.TrimSuffix(activity.decimal(math.Round(value*tenths)/tenths, digits), activity.decimalPoint()+"0")
	return activity.grouped(text) + "%"
}

func (ActivityService) tenths() float64 { return 10 }

func (ActivityService) decimalPoint() string { return "." }

func (activity *ActivityService) grouped(text string) string {
	point := activity.decimalPoint()
	whole, fraction, _ := strings.Cut(text, point)
	sign := strings.TrimRight(whole, "0123456789")
	digits := strings.TrimPrefix(whole, sign)
	size := activity.groupSize()
	groups := []string{}
	for len(digits) > size {
		groups = append([]string{digits[len(digits)-size:]}, groups...)
		digits = digits[:len(digits)-size]
	}
	return sign + strings.Join(append([]string{digits}, groups...), ",") + strings.TrimSuffix(point+fraction, point)
}

func (ActivityService) groupSize() int { return 3 }

func (activity *ActivityService) count(value float64) string {
	return activity.decimal(max(value, 0), -1)
}

func (ActivityService) decimal(value float64, digits int) string {
	return strconv.FormatFloat(value, 'f', digits, 64)
}

func (activity *ActivityService) settle() {
	activity.applicationEventPublisher.PublishEvent(ViewSettled{Source: "activity"}, false)
}
