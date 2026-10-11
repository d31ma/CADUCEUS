import (
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

// CalendarService is the calendar page: the month, day, week, agenda and split
// views, the quick-event popover and the create/edit dialog.
//
// Event sources come from the API's `source` field: `manual` events were
// created here, `invite` events were imported from an iCalendar invitation,
// `provider` events come from a connected calendar and `team` events are the
// same-domain teammates' events shown by the "Show team" overlay.
//
// Every public method ends by recomputing the view state (weeks, chips,
// labels), so the view only reads fields. Browser work that Go cannot do
// (confirm dialogs, drag and drop, the viewport, focus decoration) lives in
// CalendarBrowserService, which calls these methods.
// @Service
type CalendarService struct {
	// @Needs
	calendarClient *CalendarClient
	// @Needs
	clock Clock
	// @Needs
	secureRandom              SecureRandom
	applicationEventPublisher ApplicationEventPublisher
	TitleLabel                string
	Tabs                      []CalendarViewTab
	WeekdayLabels             []string
	Weeks                     [][]CalendarCell
	AgendaDays                []AgendaDay
	AgendaListDays            []AgendaDay
	SplitColumns              []SplitColumn
	CalendarOptions           []CalendarOption
	QuickCreateDateLabel      string
	EditingReadOnlyMessage    string
	EditingWhenLabel          string
	Form                      EventForm
	// @LocalState
	ShowTeam bool
	// @LocalState
	ViewMode string
	// @SessionState
	Cursor string
	// @SessionState
	ProviderCalendars []ProviderCalendar
	// @SessionState
	TeamEvents []CalendarEvent
	// @SessionState
	ModalOpen bool
	// @SessionState
	EditingID string
	// @SessionState
	EditingSeriesID string
	// @SessionState
	EditingRecurrenceID string
	// @SessionState
	EditScope string
	// @SessionState
	EditingReadOnly bool
	// @SessionState
	EditingOccurrence CalendarEvent
	// @SessionState
	ModalReturnSelector string
	// @SessionState
	KeyboardEventID string
	// @SessionState
	MutationKey string
	// @SessionState
	QuickCreateOpen bool
	// @SessionState
	QuickCreateDate string
	// @SessionState
	QuickCreateTitle string
	// @SessionState
	QuickCreateError string
	// @SessionState
	QuickCreateSaving bool
	events                    []CalendarEvent
	token                     string
	zone                      *time.Location
	timeZone                  string
	twentyFourHour            bool
}

// Start restores the browser's time zone and the default view.
// @OnStart
func (calendar *CalendarService) Start() {
	name, _ := calendar.clock.TimeZone()
	location, failure := time.LoadLocation(name)
	calendar.zone = calendar.zoneOr(location, failure)
	calendar.timeZone = TextUtils{}.FirstOf(name, calendar.writes().Universal)
	calendar.ViewMode = TextUtils{}.FirstOf(calendar.ViewMode, calendar.modes().Month)
	calendar.Cursor = TextUtils{}.FirstOf(calendar.Cursor, calendar.nowInstant())
	calendar.EditScope = TextUtils{}.FirstOf(calendar.EditScope, calendar.scopes().Event)
	calendar.refresh()
}

// Session keeps the bearer token of the signed-in mailbox.
// @OnEvent({"replay":true})
func (calendar *CalendarService) Session(changed SessionChanged) {
	calendar.token = changed.Token
}

// Preferences follows the 12 or 24 hour clock choice.
// @OnEvent({"replay":true})
func (calendar *CalendarService) Preferences(loaded PreferencesLoaded) {
	calendar.twentyFourHour = loaded.Use24HourClock
	calendar.refresh()
}

// Open is the page's first load: CalendarBrowserService calls it once the
// session is known, with whether the viewport is phone-sized.
func (calendar *CalendarService) Open(changed SessionChanged, narrow bool) error {
	calendar.token = changed.Token
	calendar.ViewMode = calendar.modeForViewport(narrow)
	calendar.loadProviderCalendars()
	calendar.loadEvents()
	return nil
}

// Reload fetches the visible events again.
func (calendar *CalendarService) Reload() error {
	calendar.loadEvents()
	return nil
}

// Viewport switches a wide view to the day view on a phone-sized screen.
func (calendar *CalendarService) Viewport(narrow bool) error {
	mode := calendar.modeForViewport(narrow)
	changed := mode != calendar.ViewMode
	calendar.ViewMode = mode
	calendar.reloadWhen(changed)
	return nil
}

// Shortcut runs one calendar keyboard command, such as "calendar:view-day".
func (calendar *CalendarService) Shortcut(command string) error {
	calendar.shortcutAction(command)()
	calendar.refresh()
	return nil
}

// SetViewMode shows another view and keeps the selected event in sight.
func (calendar *CalendarService) SetViewMode(mode string) error {
	calendar.switchWhenKnown(mode)
	return nil
}

// ShiftCursor moves one month, agenda page, three days, day or week, by view.
func (calendar *CalendarService) ShiftCursor(direction int) error {
	calendar.shiftWhenStep(direction)
	return nil
}

// GoToToday shows the period that contains today.
func (calendar *CalendarService) GoToToday() error {
	calendar.Cursor = calendar.nowInstant()
	calendar.loadEvents()
	return nil
}

// ToggleShowTeam shows or hides the teammates' events.
func (calendar *CalendarService) ToggleShowTeam() error {
	calendar.ShowTeam = !calendar.ShowTeam
	calendar.syncTeamEvents()
	calendar.refresh()
	return nil
}

// FocusEvent remembers an owned event as the target of move and resize commands.
func (calendar *CalendarService) FocusEvent(id string) {
	event := calendar.findEvent(calendar.events, id)
	calendar.focusWhenFound(event.ID != "", calendar.keyboardTarget(event))
	calendar.refresh()
}

// OpenCell opens the quick-event popover for a month cell, unless the click
// landed on one of the cell's event chips.
func (calendar *CalendarService) OpenCell(date string, classes string) {
	calendar.quickCreateUnlessChip(date, strings.Contains(classes, calendar.classes().ChipPrefix))
	calendar.refresh()
}

// OpenEvent opens the dialog for an event chip.
func (calendar *CalendarService) OpenEvent(id string) {
	calendar.openWhenFound(calendar.findEvent(calendar.allEvents(), id))
	calendar.refresh()
}

// OpenCreateModal opens the dialog for a new event at the next whole hour.
func (calendar *CalendarService) OpenCreateModal() {
	now := time.Now().In(calendar.zone)
	start := time.Date(now.Year(), now.Month(), now.Day(), calendar.startHour(now.Hour()), 0, 0, 0, calendar.zone)
	end := time.Date(start.Year(), start.Month(), start.Day(), start.Hour()+1, 0, 0, 0, calendar.zone)
	calendar.openNew(calendar.blankForm(start, end), calendar.createReturnSelector())
	calendar.refresh()
}

// CloseModal closes the dialog and returns focus to what opened it.
func (calendar *CalendarService) CloseModal() {
	calendar.closeModal()
	calendar.refresh()
}

// ModalBackdrop closes the dialog when the click landed on the backdrop itself.
func (calendar *CalendarService) ModalBackdrop(classes string) {
	calendar.closeModalWhen(classes == calendar.classes().ModalBackdrop)
	calendar.refresh()
}

// ModalKey closes the dialog on Escape.
func (calendar *CalendarService) ModalKey(key string) {
	calendar.closeModalWhen(key == calendar.keys().Escape)
	calendar.refresh()
}

// UpdateFormTitle keeps the dialog's title field.
func (calendar *CalendarService) UpdateFormTitle(value string) {
	calendar.Form.Title = value
}

// UpdateFormStart keeps the dialog's start field.
func (calendar *CalendarService) UpdateFormStart(value string) {
	calendar.Form.Start = value
}

// UpdateFormEnd keeps the dialog's end field.
func (calendar *CalendarService) UpdateFormEnd(value string) {
	calendar.Form.End = value
}

// UpdateFormLocation keeps the dialog's location field.
func (calendar *CalendarService) UpdateFormLocation(value string) {
	calendar.Form.Location = value
}

// UpdateFormDescription keeps the dialog's description field.
func (calendar *CalendarService) UpdateFormDescription(value string) {
	calendar.Form.Description = value
}

// UpdateFormRepeat keeps the dialog's repeat choice.
func (calendar *CalendarService) UpdateFormRepeat(value string) {
	calendar.Form.Repeat = value
}

// UpdateFormCalendar keeps the dialog's target calendar.
func (calendar *CalendarService) UpdateFormCalendar(value string) {
	calendar.Form.CalendarID = value
}

// UpdateFormAllDay switches the dialog between date and date-time fields.
func (calendar *CalendarService) UpdateFormAllDay(checked bool) {
	calendar.allDayWhenChanged(checked)
}

// UpdateEditScope switches a recurring edit between this occurrence and the
// whole series; the series rule is read from the server.
func (calendar *CalendarService) UpdateEditScope(value string) error {
	calendar.scopeWhenValid(value)
	return nil
}

// SaveEvent creates the event or saves the edit.
func (calendar *CalendarService) SaveEvent() error {
	calendar.saveWhenWritable()
	calendar.refresh()
	return nil
}

// DeleteEditing deletes the edited event in the scope the confirmations chose:
// "instance", "series", or empty for the dialog's own scope.
func (calendar *CalendarService) DeleteEditing(scope string) error {
	calendar.deleteWhenWritable(TextUtils{}.FirstOf(scope, calendar.EditScope))
	calendar.refresh()
	return nil
}

// UpdateQuickCreateTitle keeps the popover's title field.
func (calendar *CalendarService) UpdateQuickCreateTitle(value string) {
	calendar.QuickCreateTitle = value
}

// CreateQuickEvent creates a 09:00 to 10:00 event on the popover's day.
func (calendar *CalendarService) CreateQuickEvent() error {
	blocked := slices.Contains([]bool{calendar.QuickCreateDate == "", calendar.QuickCreateSaving}, true)
	calendar.quickCreateWhenReady(strings.TrimSpace(calendar.QuickCreateTitle), blocked)
	calendar.refresh()
	return nil
}

// OpenQuickCreateDetails moves from the popover to the full dialog.
func (calendar *CalendarService) OpenQuickCreateDetails() {
	day, _ := CalendarDateUtils{}.ParseLocalDate(calendar.QuickCreateDate, calendar.zone)
	calendar.closeQuickCreate(false)
	calendar.openForDate(day)
	calendar.refresh()
}

// QuickBackdrop closes the popover when the click landed on its backdrop.
func (calendar *CalendarService) QuickBackdrop(classes string) {
	calendar.closeQuickWhen(classes == calendar.classes().QuickBackdrop)
	calendar.refresh()
}

// QuickKey closes the popover on Escape.
func (calendar *CalendarService) QuickKey(key string) {
	calendar.closeQuickWhen(key == calendar.keys().Escape)
	calendar.refresh()
}

// DropOnDate moves a dragged owned event to another day, keeping its time.
func (calendar *CalendarService) DropOnDate(id string, date string) error {
	event := calendar.findEvent(calendar.events, id)
	span, valid := calendar.dateDrop(event, date)
	calendar.drop(event, date, span, valid)
	calendar.refresh()
	return nil
}

// DropOnAllDay moves a dragged owned event to a day's all-day row.
func (calendar *CalendarService) DropOnAllDay(id string, date string) error {
	event := calendar.findEvent(calendar.events, id)
	span, valid := calendar.allDayDrop(event, date)
	calendar.drop(event, date, span, valid)
	calendar.refresh()
	return nil
}

func (CalendarService) zoneOr(location *time.Location, failure error) *time.Location {
	if failure != nil {
		return time.UTC
	}
	return location
}

func (calendar *CalendarService) bearer() string {
	return "Bearer " + calendar.token
}

func (calendar *CalendarService) nowInstant() string {
	return CalendarDateUtils{}.InstantText(time.Now())
}

func (calendar *CalendarService) newKey() string {
	key, _ := calendar.secureRandom.UUID()
	return key
}

func (calendar *CalendarService) toast(message string) {
	calendar.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (calendar *CalendarService) focus(request FocusRequested) {
	calendar.applicationEventPublisher.PublishEvent(request, false)
}

func (CalendarService) modes() CalendarModes {
	return CalendarModes{
		Day: "day",
		ThreeDay: "three-day",
		WorkWeek: "work-week",
		Week: "week",
		Month: "month",
		Agenda: "agenda",
		Split: "split",
	}
}

// Loading.

func (calendar *CalendarService) loadEvents() {
	start, end := calendar.fetchRange()
	events, failure := calendar.calendarClient.EventsBetween(calendar.bearer(), CalendarDateUtils{}.InstantText(start), CalendarDateUtils{}.InstantText(end))
	calendar.events = calendar.eventsOr(events, failure)
	calendar.teamWhenShown(start, end)
	calendar.refresh()
}

func (CalendarService) eventsOr(events []CalendarEvent, failure error) []CalendarEvent {
	if failure != nil {
		return []CalendarEvent{}
	}
	return events
}

func (calendar *CalendarService) teamWhenShown(start time.Time, end time.Time) {
	if calendar.ShowTeam {
		calendar.loadTeamEvents(start, end)
	}
}

func (calendar *CalendarService) loadTeamEvents(start time.Time, end time.Time) {
	events, failure := calendar.calendarClient.TeamEvents(calendar.bearer(), CalendarDateUtils{}.InstantText(start), CalendarDateUtils{}.InstantText(end))
	calendar.TeamEvents = calendar.eventsOr(events, failure)
}

func (calendar *CalendarService) syncTeamEvents() {
	if !calendar.ShowTeam {
		calendar.TeamEvents = []CalendarEvent{}
		return
	}
	calendar.loadTeamEvents(calendar.fetchRange())
}

func (calendar *CalendarService) loadProviderCalendars() {
	list, failure := calendar.calendarClient.Calendars(calendar.bearer())
	calendar.ProviderCalendars = calendar.writableCalendars(calendar.calendarsOr(list, failure))
}

func (CalendarService) calendarsOr(list ProviderCalendarList, failure error) []ProviderCalendar {
	if failure != nil {
		return []ProviderCalendar{}
	}
	return list.Calendars
}

func (calendar *CalendarService) writableCalendars(calendars []ProviderCalendar) []ProviderCalendar {
	found := []ProviderCalendar{}
	for _, item := range calendars {
		found = calendar.keepWritable(found, item)
	}
	return found
}

func (CalendarService) keepWritable(found []ProviderCalendar, item ProviderCalendar) []ProviderCalendar {
	if item.Writable != nil && !*item.Writable {
		return found
	}
	return append(found, item)
}

func (calendar *CalendarService) fetchRange() (time.Time, time.Time) {
	cursor := calendar.cursorTime()
	days := calendar.visibleDays(cursor)
	last := days[len(days)-1]
	ranges := map[bool][]time.Time{
		true: {
			time.Date(cursor.Year(), cursor.Month()-1, calendar.bounds().MonthLead, 0, 0, 0, 0, calendar.zone),
			time.Date(cursor.Year(), cursor.Month()+1, calendar.spans().Week, calendar.bounds().LastHour, calendar.bounds().LastMinute, calendar.bounds().LastMinute, 0, calendar.zone),
		},
		false: {days[0], time.Date(last.Year(), last.Month(), last.Day(), calendar.bounds().LastHour, calendar.bounds().LastMinute, calendar.bounds().LastMinute, calendar.bounds().LastNanosecond, calendar.zone)},
	}
	chosen := ranges[calendar.ViewMode == calendar.modes().Month]
	return chosen[0], chosen[1]
}

func (calendar *CalendarService) cursorTime() time.Time {
	moment, found := CalendarDateUtils{}.ParseInstant(calendar.Cursor, calendar.zone)
	return calendar.momentOrNow(moment, found).In(calendar.zone)
}

func (CalendarService) momentOrNow(moment time.Time, found bool) time.Time {
	if !found {
		return time.Now()
	}
	return moment
}

func (calendar *CalendarService) midnight(moment time.Time) time.Time {
	local := moment.In(calendar.zone)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, calendar.zone)
}

func (calendar *CalendarService) todayDate() string {
	return CalendarDateUtils{}.CivilDate(time.Now().In(calendar.zone))
}

func (CalendarService) sources() CalendarSources {
	return CalendarSources{
		Manual: "manual",
		Invite: "invite",
		Provider: "provider",
		Team: "team",
		Local: "local",
	}
}

// The view state.

func (calendar *CalendarService) refresh() {
	cursor := calendar.cursorTime()
	all := calendar.allEvents()
	calendar.Tabs = calendar.tabs()
	calendar.TitleLabel = calendar.titleLabel(cursor)
	calendar.WeekdayLabels = calendar.weekdayLabels()
	calendar.Weeks = calendar.weeks(cursor, all)
	calendar.AgendaDays = calendar.agendaDays(cursor, all)
	calendar.AgendaListDays = calendar.daysWithEvents(calendar.AgendaDays)
	calendar.SplitColumns = calendar.splitColumns(all)
	calendar.CalendarOptions = append([]CalendarOption{{ID: calendar.sources().Local, Label: "CADUCEUS calendar"}}, calendar.providerOptions()...)
	calendar.QuickCreateDateLabel = calendar.quickDateLabel(calendar.QuickCreateDate)
	calendar.EditingReadOnlyMessage = calendar.readOnlyMessage(calendar.EditingOccurrence)
	calendar.EditingWhenLabel = calendar.whenLabel(calendar.EditingOccurrence)
}

func (calendar *CalendarService) viewModes() []CalendarViewTab {
	return []CalendarViewTab{
		{ID: calendar.modes().Day, Label: "Day"},
		{ID: calendar.modes().ThreeDay, Label: "3-Day", ClassName: calendar.classes().MobileOnly},
		{ID: calendar.modes().WorkWeek, Label: "Work Week", ClassName: calendar.classes().DesktopOnly},
		{ID: calendar.modes().Week, Label: "Week", ClassName: calendar.classes().DesktopOnly},
		{ID: calendar.modes().Month, Label: "Month"},
		{ID: calendar.modes().Agenda, Label: "Agenda", ClassName: calendar.classes().MobileOnly},
		{ID: calendar.modes().Split, Label: "Split View", ClassName: calendar.classes().DesktopOnly},
	}
}

func (calendar *CalendarService) tabs() []CalendarViewTab {
	tabs := []CalendarViewTab{}
	for _, mode := range calendar.viewModes() {
		tabs = append(tabs, calendar.tab(mode, mode.ID == calendar.ViewMode, calendar.classes().Space))
	}
	return tabs
}

func (CalendarService) tab(mode CalendarViewTab, active bool, space string) CalendarViewTab {
	mode.ClassName = "calendar-view-button " + mode.ClassName + space
	mode.Selected = "false"
	mode.TabIndex = "-1"
	if active {
		mode.ClassName += "active"
		mode.Selected = "true"
		mode.TabIndex = "0"
	}
	return mode
}

func (calendar *CalendarService) titleLabel(cursor time.Time) string {
	labels := map[string]string{
		calendar.modes().Day:   DateLabelUtils{}.WeekdayMonthDayYear(cursor),
		calendar.modes().Month: DateLabelUtils{}.MonthYear(cursor),
	}
	label, fixed := labels[calendar.ViewMode]
	return calendar.labelOr(label, fixed, calendar.rangeTitle(calendar.visibleDays(cursor)))
}

func (CalendarService) labelOr(label string, fixed bool, fallback string) string {
	if fixed {
		return label
	}
	return fallback
}

func (CalendarService) rangeTitle(days []time.Time) string {
	first := days[0]
	last := days[len(days)-1]
	right := map[bool]string{
		true:  DateLabelUtils{}.YearAndDay(last),
		false: DateLabelUtils{}.ShortMonthDayYear(last),
	}[first.Month() == last.Month()]
	return DateLabelUtils{}.ShortMonthDay(first) + " - " + right
}

// weekdayLabels formats a known Sunday in UTC, so the labels never drift a
// day in a time zone west of UTC.
func (calendar *CalendarService) weekdayLabels() []string {
	sunday := time.Date(calendar.bounds().ReferenceYear, time.September, 1, 0, 0, 0, 0, time.UTC)
	labels := []string{}
	for offset := range calendar.spans().Week {
		labels = append(labels, DateLabelUtils{}.ShortWeekday(sunday.AddDate(0, 0, offset)))
	}
	return labels
}

func (calendar *CalendarService) visibleDays(cursor time.Time) []time.Time {
	start := calendar.midnight(cursor)
	weekday := int(start.Weekday())
	plans := map[string][]int{
		calendar.modes().Day:      {0, 1},
		calendar.modes().ThreeDay: {0, calendar.spans().ThreeDays},
		calendar.modes().Agenda:   {0, calendar.spans().Agenda},
		calendar.modes().WorkWeek: {-((weekday + calendar.spans().Week - 1) % calendar.spans().Week), calendar.spans().WorkWeek},
	}
	plan, planned := plans[calendar.ViewMode]
	chosen := calendar.planOr(plan, planned, []int{-weekday, calendar.spans().Week})
	return calendar.daysFrom(start.AddDate(0, 0, chosen[0]), chosen[1])
}

func (CalendarService) planOr(plan []int, planned bool, fallback []int) []int {
	if planned {
		return plan
	}
	return fallback
}

func (CalendarService) daysFrom(start time.Time, count int) []time.Time {
	days := []time.Time{}
	for offset := range count {
		days = append(days, start.AddDate(0, 0, offset))
	}
	return days
}

// weeks is the 6 x 7 month grid that starts on the Sunday of the week holding
// the first of the cursor month, so every month keeps a stable six-row shape.
func (calendar *CalendarService) weeks(cursor time.Time, all []CalendarEvent) [][]CalendarCell {
	first := time.Date(cursor.Year(), cursor.Month(), 1, 0, 0, 0, 0, calendar.zone)
	start := first.AddDate(0, 0, -int(first.Weekday()))
	weeks := [][]CalendarCell{}
	for row := range calendar.spans().GridRows {
		weeks = append(weeks, calendar.weekRow(start.AddDate(0, 0, row*calendar.spans().Week), cursor.Month(), all))
	}
	return weeks
}

func (calendar *CalendarService) weekRow(start time.Time, month time.Month, all []CalendarEvent) []CalendarCell {
	row := []CalendarCell{}
	for _, day := range calendar.daysFrom(start, calendar.spans().Week) {
		row = append(row, calendar.cell(day, month, all))
	}
	return row
}

func (calendar *CalendarService) cell(day time.Time, month time.Month, all []CalendarEvent) CalendarCell {
	date := CalendarDateUtils{}.CivilDate(day)
	classes := []string{
		"calendar-cell",
		map[bool]string{false: "calendar-cell-other"}[day.Month() == month],
		map[bool]string{true: "calendar-cell-today"}[date == calendar.todayDate()],
	}
	return CalendarCell{
		Date:   date,
		DayLabel:  strconv.Itoa(day.Day()),
		ClassName: strings.Join(strings.Fields(strings.Join(classes, calendar.classes().Space)), calendar.classes().Space),
		Events:    calendar.chips(calendar.covering(all, date)),
	}
}

func (calendar *CalendarService) agendaDays(cursor time.Time, all []CalendarEvent) []AgendaDay {
	days := []AgendaDay{}
	for _, day := range calendar.visibleDays(cursor) {
		days = append(days, calendar.agendaDay(day, all))
	}
	return days
}

func (calendar *CalendarService) agendaDay(day time.Time, all []CalendarEvent) AgendaDay {
	date := CalendarDateUtils{}.CivilDate(day)
	events := calendar.byStart(calendar.covering(all, date))
	return AgendaDay{
		Date:      date,
		Label:        DateLabelUtils{}.ShortWeekdayMonthDay(day),
		IsToday:      date == calendar.todayDate(),
		Events:       calendar.chips(events),
		AllDayEvents: calendar.chips(calendar.withAllDay(events, true)),
		TimedEvents:  calendar.chips(calendar.withAllDay(events, false)),
	}
}

func (calendar *CalendarService) daysWithEvents(days []AgendaDay) []AgendaDay {
	found := []AgendaDay{}
	for _, day := range days {
		found = calendar.keepBusyDay(found, day)
	}
	return found
}

func (CalendarService) keepBusyDay(found []AgendaDay, day AgendaDay) []AgendaDay {
	if len(day.Events) == 0 {
		return found
	}
	return append(found, day)
}

func (calendar *CalendarService) splitColumns(all []CalendarEvent) []SplitColumn {
	owners := []string{}
	grouped := map[string][]CalendarEvent{}
	for _, event := range all {
		owners = calendar.withOwner(owners, grouped, event.OwnerLabel)
		grouped[event.OwnerLabel] = append(grouped[event.OwnerLabel], event)
	}
	return calendar.columns(owners, grouped)
}

func (CalendarService) withOwner(owners []string, grouped map[string][]CalendarEvent, owner string) []string {
	if _, seen := grouped[owner]; seen {
		return owners
	}
	return append(owners, owner)
}

func (calendar *CalendarService) columns(owners []string, grouped map[string][]CalendarEvent) []SplitColumn {
	columns := []SplitColumn{}
	for _, owner := range owners {
		columns = append(columns, SplitColumn{Owner: owner, Events: calendar.chips(calendar.byStart(grouped[owner]))})
	}
	return columns
}

func (calendar *CalendarService) covering(all []CalendarEvent, date string) []CalendarEvent {
	found := []CalendarEvent{}
	for _, event := range all {
		found = calendar.keepCovering(found, event, date)
	}
	return found
}

func (calendar *CalendarService) keepCovering(found []CalendarEvent, event CalendarEvent, date string) []CalendarEvent {
	covered := CalendarDateUtils{}.EventCoversCalendarDate(event, date, calendar.zone)
	if covered {
		return append(found, event)
	}
	return found
}

func (calendar *CalendarService) withAllDay(events []CalendarEvent, allDay bool) []CalendarEvent {
	found := []CalendarEvent{}
	for _, event := range events {
		found = calendar.keepAllDay(found, event, allDay)
	}
	return found
}

func (CalendarService) keepAllDay(found []CalendarEvent, event CalendarEvent, allDay bool) []CalendarEvent {
	if event.AllDay == allDay {
		return append(found, event)
	}
	return found
}

func (calendar *CalendarService) byStart(events []CalendarEvent) []CalendarEvent {
	sorted := append([]CalendarEvent{}, events...)
	sort.SliceStable(sorted, func(left int, right int) bool {
		return calendar.startMoment(sorted[left]).Before(calendar.startMoment(sorted[right]))
	})
	return sorted
}

func (calendar *CalendarService) startMoment(event CalendarEvent) time.Time {
	moment, _ := CalendarDateUtils{}.ParseInstant(event.Start, calendar.zone)
	return moment
}

func (calendar *CalendarService) chips(events []CalendarEvent) []CalendarChip {
	chips := []CalendarChip{}
	for _, event := range events {
		chips = append(chips, calendar.chip(event))
	}
	return chips
}

func (calendar *CalendarService) chip(event CalendarEvent) CalendarChip {
	recurrence := calendar.recurrenceLabel(event)
	return CalendarChip{
		ID:          event.ID,
		Title:       event.Title,
		TimeLabel:   calendar.timeLabel(event),
		Tooltip:     TextUtils{}.FirstOf(recurrence, event.Title),
		IsRecurring: event.IsRecurring,
		ClassName:   calendar.chipClass(event),
		Draggable:   strconv.FormatBool(!calendar.isReadOnly(event)),
		Start:       event.Start,
	}
}

func (calendar *CalendarService) timeLabel(event CalendarEvent) string {
	if event.AllDay {
		return ""
	}
	return DateLabelUtils{}.HourMinute(calendar.startMoment(event).In(calendar.zone), calendar.twentyFourHour)
}

func (CalendarService) recurrenceLabel(event CalendarEvent) string {
	labels := map[bool]string{true: "Edited occurrence", false: "Recurring event"}
	if !event.IsRecurring {
		return ""
	}
	return labels[event.IsException]
}

func (calendar *CalendarService) chipClass(event CalendarEvent) string {
	sources := map[string]string{
		calendar.sources().Invite:   "calendar-event-chip-invite",
		calendar.sources().Provider: "calendar-event-chip-provider",
		calendar.sources().Team:     "calendar-event-chip-team",
	}
	source := TextUtils{}.FirstOf(sources[event.Source], "calendar-event-chip-manual")
	recurring := map[bool]string{true: "calendar-event-chip-recurring"}[event.IsRecurring]
	selected := map[bool]string{true: "keyboard-selected"}[event.ID == calendar.KeyboardEventID]
	return strings.Join([]string{calendar.classes().ChipPrefix + "-chip", source, recurring, selected}, calendar.classes().Space)
}

func (calendar *CalendarService) isReadOnly(event CalendarEvent) bool {
	ended := map[bool]func() bool{
		true:  func() bool { return calendar.allDayEnded(event) },
		false: func() bool { return calendar.timedEnded(event) },
	}[event.AllDay]()
	return calendar.lockedOr(event, ended)
}

func (calendar *CalendarService) lockedOr(event CalendarEvent, ended bool) bool {
	if event.ReadOnly || event.Source == calendar.sources().Team || (event.Source == calendar.sources().Invite && event.Organizer != "") {
		return true
	}
	return ended
}

func (calendar *CalendarService) allDayEnded(event CalendarEvent) bool {
	return CalendarDateUtils{}.CalendarDateValue(TextUtils{}.FirstOf(event.End, event.Start)) <= calendar.todayDate()
}

func (calendar *CalendarService) timedEnded(event CalendarEvent) bool {
	moment, found := CalendarDateUtils{}.ParseInstant(TextUtils{}.FirstOf(event.End, event.Start), calendar.zone)
	return found && moment.Before(time.Now())
}

func (calendar *CalendarService) allEvents() []CalendarEvent {
	return append(calendar.ownEvents(), calendar.teamOverlay()...)
}

func (calendar *CalendarService) ownEvents() []CalendarEvent {
	own := []CalendarEvent{}
	for _, event := range calendar.events {
		event.OwnerLabel = calendar.ownOwner(event)
		event.Source = TextUtils{}.FirstOf(event.Source, calendar.sources().Manual)
		own = append(own, event)
	}
	return own
}

func (calendar *CalendarService) ownOwner(event CalendarEvent) string {
	if event.Source == calendar.sources().Provider {
		return TextUtils{}.FirstOf(calendar.joinLabels(event.AccountLabel, event.CalendarName), calendar.messages().Connected)
	}
	return "Me"
}

func (calendar *CalendarService) teamOverlay() []CalendarEvent {
	team := []CalendarEvent{}
	for _, event := range calendar.shownTeam() {
		event.OwnerLabel = TextUtils{}.FirstOf(TextUtils{}.FirstOf(event.OwnerEmail, event.UserEmail), TextUtils{}.FirstOf(event.Owner, "Team"))
		event.Source = TextUtils{}.FirstOf(event.Source, calendar.sources().Team)
		team = append(team, event)
	}
	return team
}

func (calendar *CalendarService) shownTeam() []CalendarEvent {
	if !calendar.ShowTeam {
		return []CalendarEvent{}
	}
	return calendar.TeamEvents
}

func (calendar *CalendarService) providerOptions() []CalendarOption {
	options := []CalendarOption{}
	for _, item := range calendar.ProviderCalendars {
		options = append(options, CalendarOption{
			ID:    item.CalendarID,
			Label: TextUtils{}.FirstOf(calendar.joinLabels(item.AccountLabel, item.CalendarName), calendar.messages().Connected),
			Color: TextUtils{}.FirstOf(item.CalendarColor, item.AccountColor),
		})
	}
	return options
}

func (CalendarService) joinLabels(first string, second string) string {
	return strings.Join(strings.FieldsFunc(first+"\x00"+second, func(character rune) bool { return character == 0 }), " · ")
}

func (calendar *CalendarService) quickDateLabel(date string) string {
	day, failure := CalendarDateUtils{}.ParseLocalDate(date, calendar.zone)
	if failure != nil {
		return ""
	}
	return DateLabelUtils{}.WeekdayMonthDay(day)
}

func (calendar *CalendarService) readOnlyMessage(event CalendarEvent) string {
	origin := TextUtils{}.FirstOf(calendar.joinLabels(event.AccountLabel, event.CalendarName), "the connected calendar")
	if event.Source == calendar.sources().Provider {
		return "This event is read-only from " + origin + ". Changes made at the provider will synchronize here."
	}
	return "This event is read-only because it belongs to a teammate, is an invitation, or has already ended."
}

func (calendar *CalendarService) whenLabel(event CalendarEvent) string {
	format := map[bool]func(time.Time) string{
		true:  DateLabelUtils{}.WeekdayMonthDayYear,
		false: func(moment time.Time) string { return DateLabelUtils{}.ShortDateTime(moment, calendar.twentyFourHour) },
	}[event.AllDay]
	start := calendar.formatted(event.Start, format)
	return calendar.whenText(start, calendar.formatted(event.End, format))
}

func (calendar *CalendarService) formatted(value string, format func(time.Time) string) string {
	moment, found := CalendarDateUtils{}.ParseInstant(value, calendar.zone)
	return map[bool]string{true: format(moment.In(calendar.zone))}[found]
}

func (CalendarService) whenText(start string, end string) string {
	if end != "" && end != start {
		return start + " – " + end
	}
	return start
}

func (CalendarService) scopes() EditScopes {
	return EditScopes{
		Event: "event",
		Instance: "instance",
		Series: "series",
		NoRepeat: "none",
		CustomRepeat: "custom",
	}
}

// The dialog.

func (calendar *CalendarService) blankForm(start time.Time, end time.Time) EventForm {
	return EventForm{
		Start:      CalendarDateUtils{}.LocalDateTimeValue(start),
		End:        CalendarDateUtils{}.LocalDateTimeValue(end),
		Repeat:     calendar.scopes().NoRepeat,
		TimeZone:   calendar.timeZone,
		CalendarID: calendar.sources().Local,
	}
}

func (calendar *CalendarService) startHour(hour int) int {
	if hour < calendar.bounds().Morning {
		return calendar.bounds().Morning
	}
	return hour
}

func (calendar *CalendarService) openForDate(day time.Time) {
	start := time.Date(day.Year(), day.Month(), day.Day(), calendar.bounds().Morning, 0, 0, 0, calendar.zone)
	end := time.Date(day.Year(), day.Month(), day.Day(), calendar.bounds().Morning+1, 0, 0, 0, calendar.zone)
	calendar.openNew(calendar.blankForm(start, end), calendar.ModalReturnSelector)
}

func (calendar *CalendarService) openNew(form EventForm, returnSelector string) {
	calendar.EditingID = ""
	calendar.EditingSeriesID = ""
	calendar.EditingRecurrenceID = ""
	calendar.EditScope = calendar.scopes().Event
	calendar.EditingReadOnly = false
	calendar.EditingOccurrence = CalendarEvent{}
	calendar.showModal(form, returnSelector)
}

func (calendar *CalendarService) showModal(form EventForm, returnSelector string) {
	calendar.MutationKey = calendar.newKey()
	calendar.Form = form
	calendar.ModalReturnSelector = returnSelector
	calendar.ModalOpen = true
	calendar.focus(FocusRequested{Selector: "#calendar-event-dialog", Remember: true})
}

func (calendar *CalendarService) openWhenFound(event CalendarEvent) {
	if event.ID == "" {
		return
	}
	calendar.openEvent(event)
}

func (calendar *CalendarService) openEvent(event CalendarEvent) {
	calendar.KeyboardEventID = calendar.keyboardTarget(event)
	calendar.EditingOccurrence = event
	calendar.EditingSeriesID = event.SeriesID
	calendar.EditingRecurrenceID = event.RecurrenceID
	calendar.EditScope = calendar.scopeFor(event)
	calendar.EditingID = TextUtils{}.FirstOf(event.SeriesID, event.ID)
	calendar.EditingReadOnly = calendar.isReadOnly(event)
	form := calendar.eventForm(event, EventForm{CalendarID: calendar.sources().Local})
	form.TimeZone = TextUtils{}.FirstOf(event.TimeZone, calendar.timeZone)
	calendar.showModal(form, calendar.eventSelector(event.ID))
}

func (calendar *CalendarService) scopeFor(event CalendarEvent) string {
	if event.IsRecurring {
		return calendar.scopes().Instance
	}
	return calendar.scopes().Event
}

func (calendar *CalendarService) keyboardTarget(event CalendarEvent) string {
	if event.Source == calendar.sources().Team {
		return ""
	}
	return event.ID
}

func (calendar *CalendarService) focusWhenFound(found bool, target string) {
	if found {
		calendar.KeyboardEventID = target
	}
}

// eventForm fills the dialog from an event; form keeps the fields the event does not set.
func (calendar *CalendarService) eventForm(event CalendarEvent, form EventForm) EventForm {
	end := TextUtils{}.FirstOf(event.End, event.Start)
	form.Title = event.Title
	form.Start = calendar.fieldValue(event.Start, event.AllDay)
	form.End = calendar.fieldValue(end, event.AllDay)
	form.AllDay = event.AllDay
	form.Location = event.Location
	form.Description = event.Description
	form.Repeat = calendar.repeatPreset(event.Recurrence)
	form.Recurrence = event.Recurrence
	form.TimeZone = TextUtils{}.FirstOf(event.TimeZone, calendar.writes().Universal)
	form.CalendarID = TextUtils{}.FirstOf(TextUtils{}.FirstOf(event.CalendarID, form.CalendarID), calendar.sources().Local)
	form.Error = ""
	return form
}

func (calendar *CalendarService) fieldValue(value string, allDay bool) string {
	moment, _ := CalendarDateUtils{}.ParseInstant(value, calendar.zone)
	if allDay {
		return CalendarDateUtils{}.CalendarDateValue(value)
	}
	return CalendarDateUtils{}.LocalDateTimeValue(moment.In(calendar.zone))
}

func (calendar *CalendarService) repeatPreset(rule string) string {
	preset := strings.ToLower(strings.TrimPrefix(rule, "FREQ="))
	switch {
	case rule == "":
		return calendar.scopes().NoRepeat
	case calendar.repeatRules()[preset] == rule:
		return preset
	default:
		return calendar.scopes().CustomRepeat
	}
}

func (calendar *CalendarService) repeatRule(preset string, existing string) string {
	if preset == calendar.scopes().CustomRepeat {
		return existing
	}
	return calendar.repeatRules()[preset]
}

func (CalendarService) repeatRules() map[string]string {
	return map[string]string{"daily": "FREQ=DAILY", "weekly": "FREQ=WEEKLY", "monthly": "FREQ=MONTHLY", "yearly": "FREQ=YEARLY"}
}

func (calendar *CalendarService) closeModal() {
	selector := calendar.ModalReturnSelector
	calendar.ModalOpen = false
	calendar.EditingID = ""
	calendar.EditingSeriesID = ""
	calendar.EditingRecurrenceID = ""
	calendar.EditingOccurrence = CalendarEvent{}
	calendar.EditingReadOnly = false
	calendar.ModalReturnSelector = ""
	calendar.focus(FocusRequested{Selector: selector, Restore: selector == ""})
}

func (calendar *CalendarService) closeModalWhen(close bool) {
	if close {
		calendar.closeModal()
	}
}

func (calendar *CalendarService) allDayWhenChanged(checked bool) {
	if checked == calendar.Form.AllDay {
		return
	}
	map[bool]func(){true: calendar.toAllDay, false: calendar.toTimed}[checked]()
}

func (calendar *CalendarService) toAllDay() {
	start := calendar.datePart(calendar.Form.Start)
	calendar.setAllDayWhenValid(start, calendar.allDayEnd(start, calendar.datePart(calendar.Form.End)))
}

func (CalendarService) datePart(value string) string {
	return value[:min(len(value), len("2006-01-02"))]
}

func (CalendarService) allDayEnd(start string, end string) string {
	valid := CalendarDateUtils{}.IsCalendarDate(end)
	if valid && end > start {
		return end
	}
	next, _ := CalendarDateUtils{}.AddCalendarDays(start, 1)
	return next
}

func (calendar *CalendarService) setAllDayWhenValid(start string, end string) {
	valid := CalendarDateUtils{}.IsCalendarDate(start)
	if !valid {
		return
	}
	calendar.Form.Start = start
	calendar.Form.End = end
	calendar.Form.AllDay = true
	calendar.Form.Error = ""
}

func (calendar *CalendarService) toTimed() {
	span, failure := CalendarDateUtils{}.TimedRangeForAllDayDates(EventRange{Start: calendar.Form.Start, End: calendar.Form.End}, calendar.zone)
	calendar.applyTimed(span, failure)
}

func (calendar *CalendarService) applyTimed(span EventRange, failure error) {
	calendar.Form.AllDay = false
	if failure != nil {
		return
	}
	calendar.Form.Start = span.Start
	calendar.Form.End = span.End
	calendar.Form.Error = ""
}

func (calendar *CalendarService) scopeWhenValid(value string) {
	if calendar.EditingSeriesID == "" || (value != calendar.scopes().Instance && value != calendar.scopes().Series) {
		return
	}
	calendar.changeScope(value)
}

func (calendar *CalendarService) changeScope(value string) {
	calendar.EditScope = value
	calendar.applyScope(value == calendar.scopes().Instance, calendar.EditingOccurrence.ID != "")
}

func (calendar *CalendarService) applyScope(instance bool, known bool) {
	if instance && known {
		calendar.Form = calendar.eventForm(calendar.EditingOccurrence, calendar.Form)
		return
	}
	calendar.loadSeries()
}

// loadSeries keeps the selected occurrence's dates in the dialog: the server
// applies their move and duration to the series when the whole series saves.
func (calendar *CalendarService) loadSeries() {
	master, failure := calendar.calendarClient.Event(calendar.bearer(), calendar.EditingSeriesID)
	calendar.applySeries(master, calendar.repeatPreset(master.Recurrence), failure)
}

func (calendar *CalendarService) applySeries(master CalendarEvent, preset string, failure error) {
	if failure != nil {
		calendar.Form.Error = "Could not load the series."
		return
	}
	calendar.Form.Repeat = preset
	calendar.Form.Recurrence = master.Recurrence
	calendar.Form.TimeZone = TextUtils{}.FirstOf(TextUtils{}.FirstOf(master.TimeZone, calendar.Form.TimeZone), calendar.writes().Universal)
	calendar.Form.Error = ""
}

func (calendar *CalendarService) saveWhenWritable() {
	if calendar.EditingReadOnly {
		return
	}
	calendar.validateAndSave()
}

func (calendar *CalendarService) validateAndSave() {
	span, valid := calendar.formRange(calendar.Form)
	calendar.saveWhenValid(calendar.formProblem(calendar.Form, span, valid), span)
}

func (calendar *CalendarService) formRange(form EventForm) (EventRange, bool) {
	start, startFound := CalendarDateUtils{}.ParseInstant(form.Start, calendar.zone)
	end, endFound := CalendarDateUtils{}.ParseInstant(form.End, calendar.zone)
	ranges := map[bool]EventRange{
		true:  {Start: form.Start, End: form.End, AllDay: true},
		false: {Start: CalendarDateUtils{}.InstantText(start), End: CalendarDateUtils{}.InstantText(end)},
	}
	return ranges[form.AllDay], startFound && endFound
}

// formProblem is the first failed check of the dialog, in the order the
// original form checked them, or "" when the form can be saved.
func (calendar *CalendarService) formProblem(form EventForm, span EventRange, valid bool) string {
	dates := []bool{CalendarDateUtils{}.IsCalendarDate(span.Start), CalendarDateUtils{}.IsCalendarDate(span.End), span.End > span.Start}
	checks := []bool{
		strings.TrimSpace(form.Title) == "",
		slices.Contains([]string{form.Start, form.End}, ""),
		calendar.timedCheck(form.AllDay, !valid),
		calendar.allDayCheck(form.AllDay, slices.Contains(dates, false)),
		calendar.timedCheck(form.AllDay, span.End < span.Start),
	}
	problems := []string{"Title is required", "Start and end are required", "Start and end must be valid dates", "End must be after start", "End must be on or after start"}
	return calendar.firstProblem(checks, problems)
}

func (CalendarService) timedCheck(allDay bool, failed bool) bool {
	if !allDay && failed {
		return true
	}
	return false
}

func (CalendarService) allDayCheck(allDay bool, failed bool) bool {
	if allDay && failed {
		return true
	}
	return false
}

func (CalendarService) firstProblem(checks []bool, problems []string) string {
	index := slices.Index(checks, true)
	if index < 0 {
		return ""
	}
	return problems[index]
}

func (calendar *CalendarService) saveWhenValid(problem string, span EventRange) {
	if problem != "" {
		calendar.Form.Error = problem
		return
	}
	calendar.submitForm(span)
}

func (calendar *CalendarService) submitForm(span EventRange) {
	editing := calendar.EditingID != ""
	saved := calendar.savedMessage(editing, calendar.EditScope)
	queued := map[bool]string{true: "Event update queued.", false: calendar.messages().CreationQueued}[editing]
	body := calendar.formBody(calendar.Form, span)
	scope, occurrence := calendar.scopeQuery()
	calendar.Form.Saving = true
	calendar.Form.Error = ""
	_, outcome := calendar.write(calendar.saveRequest(body, scope, occurrence), calendar.saveOffline(body, calendar.eventQuery(scope, occurrence, "")))
	calendar.afterSave(outcome, queued, saved)
}

func (calendar *CalendarService) savedMessage(editing bool, scope string) string {
	switch {
	case !editing:
		return calendar.messages().Created
	case scope == calendar.scopes().Instance:
		return "Occurrence saved."
	default:
		return "Event series saved."
	}
}

func (calendar *CalendarService) formBody(form EventForm, span EventRange) map[string]any {
	body := map[string]any{
		"title":          strings.TrimSpace(form.Title),
		"start":          span.Start,
		"end":            span.End,
		"allDay":         form.AllDay,
		"location":       strings.TrimSpace(form.Location),
		"description":    form.Description,
		"timeZone":       TextUtils{}.FirstOf(TextUtils{}.FirstOf(form.TimeZone, calendar.timeZone), calendar.writes().Universal),
		calendar.writes().IdempotencyKey: TextUtils{}.FirstOf(calendar.MutationKey, calendar.newKey()),
	}
	calendar.addTarget(body, form.CalendarID)
	calendar.addRecurrence(body, calendar.repeatRule(form.Repeat, form.Recurrence))
	return body
}

func (calendar *CalendarService) addTarget(body map[string]any, target string) {
	if calendar.EditingID == "" && target != "" && target != calendar.sources().Local {
		body["calendarId"] = target
	}
}

func (calendar *CalendarService) addRecurrence(body map[string]any, rule string) {
	if calendar.EditingID == "" || calendar.EditScope == calendar.scopes().Series {
		body["recurrence"] = rule
	}
}

func (calendar *CalendarService) scopeQuery() (string, string) {
	if calendar.EditingID != "" && calendar.EditingRecurrenceID != "" {
		return calendar.EditScope, calendar.EditingRecurrenceID
	}
	return "", ""
}

func (calendar *CalendarService) saveRequest(body map[string]any, scope string, occurrence string) func() (BehaviorValue, error) {
	id := calendar.EditingID
	if id != "" {
		return func() (BehaviorValue, error) {
			return calendar.calendarClient.Update(calendar.bearer(), id, scope, occurrence, body)
		}
	}
	return func() (BehaviorValue, error) { return calendar.calendarClient.Create(calendar.bearer(), body) }
}

func (calendar *CalendarService) saveOffline(body map[string]any, query string) OfflineWriteRequested {
	if calendar.EditingID != "" {
		return calendar.offline(calendar.eventPath(calendar.EditingID)+query, calendar.writes().Put, body, "Update calendar event")
	}
	return calendar.offline(calendar.eventsPath(), calendar.writes().Post, body, calendar.messages().CreateEvent)
}

func (calendar *CalendarService) afterSave(outcome string, queued string, saved string) {
	switch outcome {
	case calendar.writes().Queued:
		calendar.closeModal()
		calendar.toast(queued)
	case calendar.writes().Saved:
		calendar.closeModal()
		calendar.toast(saved)
		calendar.loadEvents()
	default:
		calendar.Form.Saving = false
		calendar.Form.Error = "Save failed"
	}
}

func (calendar *CalendarService) deleteWhenWritable(scope string) {
	if calendar.EditingID == "" || calendar.EditingReadOnly {
		return
	}
	calendar.deleteEditing(scope)
}

func (calendar *CalendarService) deleteEditing(scope string) {
	key := TextUtils{}.FirstOf(calendar.MutationKey, calendar.newKey())
	id := calendar.EditingID
	occurrence := map[bool]string{true: calendar.EditingRecurrenceID}[scope == calendar.scopes().Instance]
	query := map[bool]string{true: scope}[scope != calendar.scopes().Event]
	send := func() (BehaviorValue, error) {
		return calendar.calendarClient.Delete(calendar.bearer(), id, query, occurrence, key)
	}
	offline := calendar.offline(calendar.eventPath(id)+calendar.eventQuery(query, occurrence, key), "DELETE", nil, "Delete calendar event")
	_, outcome := calendar.write(send, offline)
	calendar.afterDelete(outcome, calendar.deletedMessage(scope))
}

func (calendar *CalendarService) deletedMessage(scope string) string {
	switch scope {
	case calendar.scopes().Instance:
		return "Occurrence deleted. The series remains."
	case calendar.scopes().Series:
		return "Event series deleted."
	default:
		return "Event deleted."
	}
}

func (calendar *CalendarService) afterDelete(outcome string, deleted string) {
	switch outcome {
	case calendar.writes().Queued:
		calendar.closeModal()
		calendar.toast("Event deletion queued.")
	case calendar.writes().Saved:
		calendar.closeModal()
		calendar.toast(deleted)
		calendar.loadEvents()
	default:
		calendar.toast("Delete failed.")
	}
}

func (CalendarService) keys() CalendarKeys {
	return CalendarKeys{
		Up: "up",
		Down: "down",
		Left: "left",
		Right: "right",
		Escape: "Escape",
	}
}

// Writes queue only transport failures; an error status stays visible.

func (calendar *CalendarService) write(send func() (BehaviorValue, error), offline OfflineWriteRequested) (BehaviorValue, string) {
	answer, failure := send()
	outcome := calendar.outcomeOf(failure)
	calendar.queueWhen(outcome == calendar.writes().Queued, offline)
	return answer, outcome
}

func (calendar *CalendarService) outcomeOf(failure error) string {
	switch {
	case failure == nil:
		return calendar.writes().Saved
	case HTTPUtils{}.Refused(failure):
		return "failed"
	default:
		return calendar.writes().Queued
	}
}

func (calendar *CalendarService) queueWhen(queued bool, offline OfflineWriteRequested) {
	if queued {
		calendar.applicationEventPublisher.PublishEvent(offline, false)
	}
}

func (calendar *CalendarService) offline(path string, method string, body any, message string) OfflineWriteRequested {
	return OfflineWriteRequested{
		Kind:        calendar.writes().Kind,
		Path:        path,
		Method:      method,
		Body:        body,
		Message:     message,
		CoalesceKey: calendar.coalesceKey(method),
		Metadata:    map[string]any{calendar.writes().Kind: true},
	}
}

func (calendar *CalendarService) coalesceKey(method string) string {
	if method == calendar.writes().Post {
		return ""
	}
	return calendar.writes().CoalescePrefix + calendar.writeTarget()
}

func (calendar *CalendarService) writeTarget() string {
	return TextUtils{}.FirstOf(calendar.EditingID, calendar.KeyboardEventID)
}

func (calendar *CalendarService) eventsPath() string {
	return "/calendar/events"
}

func (calendar *CalendarService) eventPath(id string) string {
	return calendar.eventsPath() + "/" + url.PathEscape(id)
}

func (calendar *CalendarService) eventQuery(scope string, occurrence string, key string) string {
	values := []string{}
	for _, pair := range [][2]string{{"scope", scope}, {"occurrenceStart", occurrence}, {calendar.writes().IdempotencyKey, key}} {
		values = calendar.withQuery(values, pair)
	}
	return calendar.queryText(values)
}

func (CalendarService) withQuery(values []string, pair [2]string) []string {
	if pair[1] == "" {
		return values
	}
	return append(values, pair[0]+"="+url.QueryEscape(pair[1]))
}

func (CalendarService) queryText(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return "?" + strings.Join(values, "&")
}

func (CalendarService) classes() CalendarClasses {
	return CalendarClasses{
		MobileOnly: "calendar-mobile-only",
		DesktopOnly: "calendar-desktop-only",
		ModalBackdrop: "event-modal-backdrop",
		QuickBackdrop: "quick-event-backdrop",
		ChipPrefix: "calendar-event",
		Space: " ",
	}
}

// Quick event popover.

func (calendar *CalendarService) quickCreateUnlessChip(date string, chip bool) {
	if chip {
		return
	}
	calendar.openQuickCreate(date)
}

func (calendar *CalendarService) openQuickCreate(date string) {
	calendar.QuickCreateDate = date
	calendar.QuickCreateTitle = ""
	calendar.QuickCreateError = ""
	calendar.QuickCreateSaving = false
	calendar.QuickCreateOpen = true
	calendar.ModalReturnSelector = calendar.dateSelector(date)
	calendar.focus(FocusRequested{Selector: ".quick-event-popover input", Remember: true})
}

func (calendar *CalendarService) closeQuickCreate(restore bool) {
	calendar.QuickCreateOpen = false
	calendar.restoreWhen(restore)
}

func (calendar *CalendarService) restoreWhen(restore bool) {
	if restore {
		calendar.focus(FocusRequested{Restore: true})
	}
}

func (calendar *CalendarService) closeQuickWhen(close bool) {
	if close {
		calendar.closeQuickCreate(true)
	}
}

func (calendar *CalendarService) quickCreateWhenReady(title string, blocked bool) {
	switch {
	case title == "":
		calendar.QuickCreateError = "Title is required"
	case blocked:
		return
	default:
		calendar.submitQuick(title)
	}
}

func (calendar *CalendarService) submitQuick(title string) {
	day, _ := CalendarDateUtils{}.ParseLocalDate(calendar.QuickCreateDate, calendar.zone)
	start := time.Date(day.Year(), day.Month(), day.Day(), calendar.bounds().Morning, 0, 0, 0, calendar.zone)
	end := time.Date(day.Year(), day.Month(), day.Day(), calendar.bounds().Morning+1, 0, 0, 0, calendar.zone)
	body := map[string]any{
		"title":          title,
		"start":          CalendarDateUtils{}.InstantText(start),
		"end":            CalendarDateUtils{}.InstantText(end),
		"timeZone":       calendar.timeZone,
		calendar.writes().IdempotencyKey: calendar.newKey(),
	}
	calendar.QuickCreateSaving = true
	send := func() (BehaviorValue, error) { return calendar.calendarClient.Create(calendar.bearer(), body) }
	_, outcome := calendar.write(send, calendar.offline(calendar.eventsPath(), calendar.writes().Post, body, calendar.messages().CreateEvent))
	calendar.afterQuick(outcome)
}

func (calendar *CalendarService) afterQuick(outcome string) {
	switch outcome {
	case calendar.writes().Queued:
		calendar.closeQuickCreate(false)
		calendar.toast(calendar.messages().CreationQueued)
	case calendar.writes().Saved:
		calendar.closeQuickCreate(false)
		calendar.toast(calendar.messages().Created)
		calendar.loadEvents()
	default:
		calendar.QuickCreateSaving = false
		calendar.QuickCreateError = "Create failed"
	}
}

func (CalendarService) writes() CalendarWrites {
	return CalendarWrites{
		Put: "PUT",
		Post: "POST",
		Saved: "saved",
		Queued: "queued",
		Kind: "calendar",
		CoalescePrefix: "calendar:",
		IdempotencyKey: "idempotencyKey",
		Universal: "UTC",
	}
}

// Moving events by drag and by keyboard.

func (calendar *CalendarService) dateDrop(event CalendarEvent, date string) (EventRange, bool) {
	span := EventRange{Start: event.Start, End: TextUtils{}.FirstOf(event.End, event.Start)}
	shifted, shiftFailure := CalendarDateUtils{}.ShiftAllDayRangeToDate(span, date)
	moved, moveFailure := CalendarDateUtils{}.MoveTimedRangeToDate(span, date, calendar.zone)
	sameDay := map[bool]bool{
		true:  shifted.Start == CalendarDateUtils{}.CalendarDateValue(event.Start),
		false: CalendarDateUtils{}.CivilDate(calendar.startMoment(event).In(calendar.zone)) == date,
	}[event.AllDay]
	next := map[bool]EventRange{true: shifted, false: moved}[event.AllDay]
	failed := map[bool]bool{true: shiftFailure != nil, false: moveFailure != nil}[event.AllDay]
	return next, calendar.movable(sameDay, failed)
}

func (CalendarService) movable(sameDay bool, failed bool) bool {
	return !sameDay && !failed
}

// allDayDrop turns a timed event into a one-day all-day event; a recurring
// timed occurrence only moves, because converting it would change the
// recurrence identity and orphan the exception.
func (calendar *CalendarService) allDayDrop(event CalendarEvent, date string) (EventRange, bool) {
	span := EventRange{Start: event.Start, End: TextUtils{}.FirstOf(event.End, event.Start)}
	shifted, shiftFailure := CalendarDateUtils{}.ShiftAllDayRangeToDate(span, date)
	moved, moveFailure := CalendarDateUtils{}.MoveTimedRangeToDate(span, date, calendar.zone)
	whole, wholeFailure := CalendarDateUtils{}.AllDayRangeForDate(date)
	kind := calendar.allDayDropKind(event)
	next := map[string]EventRange{"shift": shifted, "move": moved, "whole": whole}[kind]
	failure := map[string]error{"shift": shiftFailure, "move": moveFailure, "whole": wholeFailure}[kind]
	return next, failure == nil
}

func (CalendarService) allDayDropKind(event CalendarEvent) string {
	switch {
	case event.AllDay:
		return "shift"
	case event.IsRecurring:
		return "move"
	default:
		return "whole"
	}
}

func (calendar *CalendarService) drop(event CalendarEvent, date string, span EventRange, valid bool) {
	message := calendar.dropMessage(event, span)
	allowed := calendar.dropAllowed(event, date, valid)
	calendar.persistWhen(allowed, event, calendar.timesBody(span), message)
}

func (calendar *CalendarService) dropAllowed(event CalendarEvent, date string, valid bool) bool {
	if event.ID == "" || date == "" || !valid || calendar.isReadOnly(event) {
		return false
	}
	return true
}

func (calendar *CalendarService) persistWhen(allowed bool, event CalendarEvent, body map[string]any, message string) {
	if allowed {
		calendar.persistTimes(event, body, message)
	}
}

func (calendar *CalendarService) dropMessage(event CalendarEvent, span EventRange) string {
	if span.AllDay && !event.AllDay {
		return "Event moved to all day."
	}
	return calendar.messages().Moved
}

func (calendar *CalendarService) timesBody(span EventRange) map[string]any {
	return map[string]any{"start": span.Start, "end": span.End, "allDay": span.AllDay, calendar.writes().IdempotencyKey: calendar.newKey()}
}

func (calendar *CalendarService) persistTimes(event CalendarEvent, body map[string]any, message string) {
	id := TextUtils{}.FirstOf(event.SeriesID, event.ID)
	scope, occurrence := calendar.occurrenceQuery(event)
	send := func() (BehaviorValue, error) {
		return calendar.calendarClient.Update(calendar.bearer(), id, scope, occurrence, body)
	}
	offline := calendar.offline(calendar.eventPath(id)+calendar.eventQuery(scope, occurrence, ""), calendar.writes().Put, body, message)
	offline.CoalesceKey = calendar.writes().CoalescePrefix + id
	_, outcome := calendar.write(send, offline)
	calendar.afterMove(outcome, message)
}

func (calendar *CalendarService) occurrenceQuery(event CalendarEvent) (string, string) {
	if event.IsRecurring {
		return calendar.scopes().Instance, TextUtils{}.FirstOf(event.RecurrenceID, event.Start)
	}
	return "", ""
}

func (calendar *CalendarService) afterMove(outcome string, message string) {
	switch outcome {
	case calendar.writes().Queued:
		calendar.toast(strings.TrimSuffix(message, ".") + " queued.")
	case calendar.writes().Saved:
		calendar.toast(message)
		calendar.loadEvents()
	default:
		calendar.toast("Could not update event.")
	}
}

// moveSelected follows Mailspring: arrows move and Shift+arrows resize;
// vertical steps are 15 minutes in the timed views and horizontal steps are
// one day. Vertical commands do nothing in the month and agenda views.
func (calendar *CalendarService) moveSelected(move EventMove) {
	event := calendar.findEvent(calendar.events, calendar.KeyboardEventID)
	calendar.moveWhenWritable(event, move)
}

func (calendar *CalendarService) moveWhenWritable(event CalendarEvent, move EventMove) {
	if event.ID == "" || calendar.isReadOnly(event) {
		return
	}
	map[bool]func(CalendarEvent, EventMove){true: calendar.moveAllDay, false: calendar.moveTimed}[event.AllDay](event, move)
}

func (calendar *CalendarService) moveAllDay(event CalendarEvent, move EventMove) {
	amount := map[string]int{calendar.keys().Left: -1, calendar.keys().Right: 1}[move.Direction]
	current, _ := CalendarDateUtils{}.ShiftAllDayRangeToDate(EventRange{Start: event.Start, End: TextUtils{}.FirstOf(event.End, event.Start)}, CalendarDateUtils{}.CalendarDateValue(event.Start))
	resizedEnd, _ := CalendarDateUtils{}.AddCalendarDays(current.End, amount)
	shiftedStart, _ := CalendarDateUtils{}.AddCalendarDays(current.Start, amount)
	shifted, _ := CalendarDateUtils{}.ShiftAllDayRangeToDate(current, shiftedStart)
	next := map[bool]EventRange{true: {Start: current.Start, End: resizedEnd, AllDay: true}, false: shifted}[move.Resize]
	length, _ := CalendarDateUtils{}.CalendarDaysBetween(next.Start, next.End)
	checks := []bool{!calendar.verticalDirections()[move.Direction], length >= 1}
	calendar.persistWhen(!slices.Contains(checks, false), event, calendar.timesBody(next), calendar.moveMessage(move.Resize))
}

func (calendar *CalendarService) moveTimed(event CalendarEvent, move EventMove) {
	start, startFound := CalendarDateUtils{}.ParseInstant(event.Start, calendar.zone)
	end, endFound := CalendarDateUtils{}.ParseInstant(TextUtils{}.FirstOf(event.End, event.Start), calendar.zone)
	minutes := map[string]int{calendar.keys().Up: -calendar.spans().StepMinutes, calendar.keys().Down: calendar.spans().StepMinutes}[move.Direction]
	days := map[string]int{calendar.keys().Left: -1, calendar.keys().Right: 1}[move.Direction]
	nextStart := map[bool]time.Time{true: start, false: calendar.shifted(start, minutes, days)}[move.Resize]
	nextEnd := calendar.shifted(end, minutes, days)
	checks := []bool{startFound, endFound, calendar.directionAllowed(move.Direction), calendar.longEnough(move.Resize, nextStart, nextEnd)}
	body := map[string]any{"start": CalendarDateUtils{}.InstantText(nextStart), "end": CalendarDateUtils{}.InstantText(nextEnd), calendar.writes().IdempotencyKey: calendar.newKey()}
	calendar.persistWhen(!slices.Contains(checks, false), event, body, calendar.moveMessage(move.Resize))
}

func (calendar *CalendarService) directionAllowed(direction string) bool {
	return calendar.timedModes()[calendar.ViewMode] || !calendar.verticalDirections()[direction]
}

func (calendar *CalendarService) longEnough(resize bool, start time.Time, end time.Time) bool {
	return !resize || end.Sub(start) >= time.Duration(calendar.spans().StepMinutes)*time.Minute
}

func (calendar *CalendarService) shifted(moment time.Time, minutes int, days int) time.Time {
	local := moment.In(calendar.zone)
	return local.AddDate(0, 0, days).Add(time.Duration(minutes) * time.Minute)
}

func (calendar *CalendarService) moveMessage(resize bool) string {
	return map[bool]string{true: "Event resized.", false: calendar.messages().Moved}[resize]
}

func (calendar *CalendarService) shortcutAction(command string) func() {
	actions := map[string]func(){
		"calendar:move-event-up":      func() { calendar.moveSelected(EventMove{Direction: calendar.keys().Up}) },
		"calendar:move-event-down":    func() { calendar.moveSelected(EventMove{Direction: calendar.keys().Down}) },
		"calendar:move-event-left":    func() { calendar.moveSelected(EventMove{Direction: calendar.keys().Left}) },
		"calendar:move-event-right":   func() { calendar.moveSelected(EventMove{Direction: calendar.keys().Right}) },
		"calendar:resize-event-up":    func() { calendar.moveSelected(EventMove{Direction: calendar.keys().Up, Resize: true}) },
		"calendar:resize-event-down":  func() { calendar.moveSelected(EventMove{Direction: calendar.keys().Down, Resize: true}) },
		"calendar:resize-event-left":  func() { calendar.moveSelected(EventMove{Direction: calendar.keys().Left, Resize: true}) },
		"calendar:resize-event-right": func() { calendar.moveSelected(EventMove{Direction: calendar.keys().Right, Resize: true}) },
		"calendar:view-day":           func() { calendar.switchWhenKnown(calendar.modes().Day) },
		"calendar:view-week":          func() { calendar.switchWhenKnown(calendar.modes().Week) },
		"calendar:view-month":         func() { calendar.switchWhenKnown(calendar.modes().Month) },
		"calendar:view-agenda":        func() { calendar.switchWhenKnown(calendar.modes().Agenda) },
		"calendar:go-to-today":        func() { calendar.Cursor = calendar.nowInstant(); calendar.loadEvents() },
		"calendar:navigate-next":      func() { calendar.shiftWhenStep(1) },
		"calendar:navigate-previous":  func() { calendar.shiftWhenStep(-1) },
		"calendar:refresh-calendars":  calendar.loadEvents,
	}
	return calendar.actionOr(actions, command)
}

func (CalendarService) actionOr(actions map[string]func(), command string) func() {
	action, known := actions[command]
	if !known {
		return func() {}
	}
	return action
}

func (CalendarService) messages() CalendarMessages {
	return CalendarMessages{
		Moved: "Event moved.",
		Created: "Event created.",
		CreationQueued: "Event creation queued.",
		CreateEvent: "Create calendar event",
		Connected: "Connected calendar",
	}
}

// Navigation.

func (calendar *CalendarService) switchWhenKnown(mode string) {
	if !calendar.knownModes()[mode] {
		return
	}
	calendar.switchView(mode)
}

func (calendar *CalendarService) switchView(mode string) {
	selected := calendar.findEvent(calendar.events, calendar.KeyboardEventID)
	cursor, found := CalendarDateUtils{}.CalendarCursorForEvent(selected, calendar.zone)
	calendar.Cursor = calendar.cursorFrom(cursor, found)
	calendar.ViewMode = mode
	calendar.ShowTeam = calendar.teamForMode(mode)
	calendar.loadEvents()
	calendar.revealWhen(calendar.KeyboardEventID)
}

func (calendar *CalendarService) cursorFrom(cursor time.Time, found bool) string {
	if found {
		return CalendarDateUtils{}.InstantText(cursor)
	}
	return calendar.Cursor
}

func (calendar *CalendarService) teamForMode(mode string) bool {
	if mode == calendar.modes().Split {
		return true
	}
	return calendar.ShowTeam
}

func (calendar *CalendarService) revealWhen(id string) {
	if id != "" {
		calendar.applicationEventPublisher.PublishEvent(EventRevealRequested{EventID: id}, false)
	}
}

func (calendar *CalendarService) shiftWhenStep(direction int) {
	if direction != -1 && direction != 1 {
		return
	}
	calendar.shiftBy(direction)
}

func (calendar *CalendarService) shiftBy(direction int) {
	steps := map[string][]int{
		calendar.modes().Month:    {1, 0},
		calendar.modes().Agenda:   {0, calendar.spans().Agenda},
		calendar.modes().ThreeDay: {0, calendar.spans().ThreeDays},
		calendar.modes().Day:      {0, 1},
	}
	step, known := steps[calendar.ViewMode]
	chosen := calendar.planOr(step, known, []int{0, calendar.spans().Week})
	calendar.Cursor = CalendarDateUtils{}.InstantText(calendar.cursorTime().AddDate(0, chosen[0]*direction, chosen[1]*direction))
	calendar.loadEvents()
}

func (calendar *CalendarService) modeForViewport(narrow bool) string {
	if narrow && calendar.wideModes()[calendar.ViewMode] {
		return calendar.modes().Day
	}
	return calendar.ViewMode
}

func (calendar *CalendarService) reloadWhen(changed bool) {
	if changed {
		calendar.loadEvents()
	}
}

func (calendar *CalendarService) findEvent(events []CalendarEvent, id string) CalendarEvent {
	found := CalendarEvent{}
	for _, event := range events {
		found = calendar.firstMatch(found, event, id)
	}
	return found
}

func (CalendarService) firstMatch(found CalendarEvent, candidate CalendarEvent, id string) CalendarEvent {
	if found.ID == "" && id != "" && candidate.ID == id {
		return candidate
	}
	return found
}

func (CalendarService) spans() CalendarSpans {
	return CalendarSpans{
		Week: 7,
		WorkWeek: 5,
		ThreeDays: 3,
		Agenda: 14,
		GridRows: 6,
		StepMinutes: 15,
	}
}

// Selectors that return focus to what opened a dialog.

func (CalendarService) createReturnSelector() string {
	return "[data-calendar-create] button"
}

func (CalendarService) eventSelector(id string) string {
	return `[data-calendar-event-id="` + strings.ReplaceAll(id, `"`, `\"`) + `"]`
}

func (CalendarService) dateSelector(date string) string {
	return `[data-calendar-date="` + date + `"]`
}

func (CalendarService) bounds() DayBounds {
	return DayBounds{
		Morning: 9,
		LastHour: 23,
		LastMinute: 59,
		LastNanosecond: 999_000_000,
		MonthLead: 25,
		ReferenceYear: 2024,
	}
}


func (calendar *CalendarService) knownModes() map[string]bool {
	known := map[string]bool{}
	for _, mode := range calendar.viewModes() {
		known[mode.ID] = true
	}
	return known
}

func (calendar *CalendarService) wideModes() map[string]bool {
	return map[string]bool{calendar.modes().WorkWeek: true, calendar.modes().Week: true, calendar.modes().Split: true}
}

func (calendar *CalendarService) timedModes() map[string]bool {
	return map[string]bool{calendar.modes().Day: true, calendar.modes().ThreeDay: true, calendar.modes().WorkWeek: true, calendar.modes().Week: true}
}

func (calendar *CalendarService) verticalDirections() map[string]bool {
	return map[string]bool{calendar.keys().Up: true, calendar.keys().Down: true}
}

