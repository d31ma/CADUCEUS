import "time"

// InviteUtils writes when a meeting invitation happens, as the reader's
// inviteWhen did with Intl: an all-day range shows floating UTC dates and
// treats the end date as exclusive; a timed range shows local times.
type InviteUtils struct{}

// When is "Mon, Oct 12 · All day", "Mon, Oct 12 – Wed, Oct 14 · All day",
// "Mon, Oct 12, 9:00 AM – 10:00 AM" or "" for no start.
func (InviteUtils) When(invite CalendarInvite) string {
	start, startFailure := time.Parse(time.RFC3339Nano, invite.Start)
	end, endFailure := time.Parse(time.RFC3339Nano, invite.End)
	end = map[bool]time.Time{true: start, false: end}[endFailure != nil]
	ranges := map[bool]func(time.Time, time.Time) string{true: InviteUtils{}.allDay, false: InviteUtils{}.timed}
	return map[bool]string{true: "", false: ranges[invite.AllDay](start, end)}[startFailure != nil]
}

func (InviteUtils) allDay(start time.Time, end time.Time) string {
	first := start.UTC()
	last := end.UTC().Add(-time.Millisecond * time.Duration(map[bool]int{true: 1}[end.After(start)]))
	return InviteUtils{}.days(first, last) + " · All day"
}

func (InviteUtils) days(first time.Time, last time.Time) string {
	layout := "Mon, Jan 2"
	if first.Format(time.DateOnly) == last.Format(time.DateOnly) {
		return first.Format(layout)
	}
	return first.Format(layout) + InviteUtils{}.dash() + last.Format(layout)
}

func (InviteUtils) timed(start time.Time, end time.Time) string {
	layout := "Mon, Jan 2, 3:04 PM"
	local := end.Local()
	sameDay := start.Local().Format(time.DateOnly) == local.Format(time.DateOnly)
	ending := map[bool]string{true: local.Format("3:04 PM"), false: local.Format(layout)}[sameDay]
	return start.Local().Format(layout) + map[bool]string{true: "", false: InviteUtils{}.dash() + ending}[start.Equal(end)]
}

func (InviteUtils) dash() string { return " – " }
