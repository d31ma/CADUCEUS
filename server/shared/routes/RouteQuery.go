import "net/http"

// RouteQuery reads a request's query string as the JavaScript routes did
// with Object.fromEntries(url.searchParams): the last value of a repeated
// name wins, and a missing name reads as "".
type RouteQuery struct{}

// Value answers one query parameter.
func (RouteQuery) Value(request *http.Request, name string) string {
	values := request.URL.Query()[name]
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

// Has reports whether the query names a parameter.
func (RouteQuery) Has(request *http.Request, name string) bool {
	_, present := request.URL.Query()[name]
	return present
}
