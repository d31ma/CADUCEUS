// CalendarEventDropped is a dragged event chip dropped on a day, or on a day's
// all-day row when AllDay is true.
type CalendarEventDropped struct {
	ID     string `json:"id"`
	Date   string `json:"date"`
	AllDay bool   `json:"allDay"`
}
