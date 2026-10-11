// CalendarDeleteChosen is the answer to the delete confirmation: "instance" or
// "series" for a recurring event, or "" for the dialog's own scope.
type CalendarDeleteChosen struct {
	Scope string `json:"scope"`
}
