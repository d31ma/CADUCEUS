import (
	"net/url"
	"strings"
)

// RouteUtils finds the message that a reader route names: /email/<id>, or
// the email or id query value.
type RouteUtils struct{}

func (RouteUtils) EmailId(path string, search string) string {
	return RouteUtils{}.emailId(path, search)
}

func (RouteUtils) prefix() string { return "/email/" }

func (RouteUtils) IsStandalone(path string) bool {
	return strings.HasPrefix(path, RouteUtils{}.prefix())
}

func (RouteUtils) emailId(path string, search string) string {
	if strings.HasPrefix(path, (RouteUtils{}).prefix()) {
		return RouteUtils{}.decoded(strings.TrimPrefix(path, RouteUtils{}.prefix()))
	}
	return RouteUtils{}.queryEmailId(search)
}

func (RouteUtils) decoded(value string) string {
	text, failure := url.PathUnescape(value)
	if failure != nil {
		return value
	}
	return text
}

func (RouteUtils) queryEmailId(search string) string {
	values, _ := url.ParseQuery(strings.TrimPrefix(search, "?"))
	return TextUtils{}.FirstOf(values.Get("email"), values.Get("id"))
}
