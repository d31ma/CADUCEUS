import (
	"encoding/json"
	"net/url"
	"slices"
	"strings"
)

// LaunchUtils reads the composer prefill that the installed app's manifest
// hands over: a mailto protocol launch (/compose?protocol=mailto:...) or a
// share-target launch (/compose?shareTitle=&shareText=&shareUrl=). Only these
// manifest-owned parameters count, so an ordinary link cannot fill in
// recipients. It is the Go home of shared/scripts/pwa-launch.js.
type LaunchUtils struct{}

// ComposePrefill merges the prefill another view stored for the composer
// (JSON text) with the prefill of a compose launch address query.
func (LaunchUtils) ComposePrefill(stored string, search string) map[string]any {
	query := LaunchUtils{}.query(search)
	fields := LaunchUtils{}.fields()
	launched := map[bool]func() map[string]string{
		true:  func() map[string]string { return LaunchUtils{}.mailto(query.Get(fields.Protocol), fields) },
		false: func() map[string]string { return LaunchUtils{}.shared(query, fields) },
	}[query.Get(fields.Protocol) != ""]()
	return LaunchUtils{}.merged(stored, launched)
}

// IsComposeLaunch is true for an address that carries manifest launch
// parameters.
func (LaunchUtils) IsComposeLaunch(search string) bool {
	query := LaunchUtils{}.query(search)
	fields := LaunchUtils{}.fields()
	return slices.ContainsFunc([]string{fields.Protocol, fields.Title, fields.Text, fields.Address}, query.Has)
}

func (LaunchUtils) mailto(value string, fields LaunchFields) map[string]string {
	link, failure := url.Parse(value)
	if failure != nil || link.Scheme != "mailto" {
		return map[string]string{}
	}
	return LaunchUtils{}.filled(map[string]string{
		fields.To: LaunchUtils{}.recipients(link.Opaque + link.Path), fields.Cc: LaunchUtils{}.recipients(link.Query().Get(fields.Cc)),
		fields.Subject: LaunchUtils{}.bounded(link.Query().Get(fields.Subject), fields.SubjectMax), fields.Body: LaunchUtils{}.bounded(link.Query().Get(fields.Body), LaunchUtils{}.longest()),
	})
}

func (LaunchUtils) shared(query url.Values, fields LaunchFields) map[string]string {
	parts := []string{LaunchUtils{}.bounded(query.Get(fields.Text), LaunchUtils{}.longest()), LaunchUtils{}.webAddress(LaunchUtils{}.bounded(query.Get(fields.Address), 8192))}
	return LaunchUtils{}.filled(map[string]string{
		fields.Subject: LaunchUtils{}.bounded(query.Get(fields.Title), fields.SubjectMax),
		fields.Body:    strings.Join(slices.DeleteFunc(parts, func(part string) bool { return part == "" }), "\n\n"),
	})
}

func (LaunchUtils) webAddress(value string) string {
	link, failure := url.Parse(value)
	if failure != nil || !slices.Contains([]string{"http", "https"}, link.Scheme) {
		return ""
	}
	return link.String()
}

func (LaunchUtils) recipients(value string) string {
	unescaped, _ := url.PathUnescape(value)
	parts := strings.Split(unescaped, ",")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	kept := slices.DeleteFunc(parts, func(part string) bool { return part == "" })
	return strings.Join(kept[:min(100, len(kept))], ", ")
}

func (LaunchUtils) bounded(value string, limit int) string {
	return strings.TrimSpace(string([]rune(value)[:min(limit, len([]rune(value)))]))
}

func (LaunchUtils) filled(values map[string]string) map[string]string {
	for name, value := range values {
		delete(values, map[bool]string{true: name}[value == ""])
	}
	delete(values, "")
	return values
}

func (LaunchUtils) merged(stored string, launched map[string]string) map[string]any {
	fields := map[string]any{}
	json.Unmarshal([]byte(stored), &fields)
	for name, value := range launched {
		fields[name] = value
	}
	return fields
}

func (LaunchUtils) query(search string) url.Values {
	query, _ := url.ParseQuery(strings.TrimPrefix(search, "?"))
	return query
}

func (LaunchUtils) longest() int { return 100000 }

func (LaunchUtils) fields() LaunchFields {
	return LaunchFields{To: "to", Cc: "cc", Subject: "subject", Body: "body", Protocol: "protocol", Title: "shareTitle", Text: "shareText", Address: "shareUrl", SubjectMax: 998}
}
