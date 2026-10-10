import "net/http"

// RouteQuery reads a request's query string as the JavaScript routes did:
// one text per name, the last one when a name repeats.
type RouteQuery struct{}

// Of answers the query values of a request.
func (RouteQuery) Of(request *http.Request) map[string]string {
	values := map[string]string{}
	for name, list := range request.URL.Query() {
		if len(list) > 0 {
			values[name] = list[len(list)-1]
		}
	}
	return values
}

// Has reports whether the query names a value.
func (RouteQuery) Has(query map[string]string, name string) bool {
	_, found := query[name]
	return found
}
