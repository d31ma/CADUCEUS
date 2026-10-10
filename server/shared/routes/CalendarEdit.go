import "net/http"

// CalendarEdit is one change request to an event: who asks, the request and
// its body, the event it found (local with its document id, or from a
// provider), and the scope and occurrence the query names.
type CalendarEdit struct {
	Caller          RouteCaller
	Request         *http.Request
	Body            map[string]any
	DocumentId      string
	Event           map[string]any
	Provider        bool
	Scope           string
	OccurrenceStart string
}
