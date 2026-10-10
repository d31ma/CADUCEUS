import (
	"net/http"
	"sort"
	"strings"
)

// PresenceRoute answers /presence/heartbeat and /presence/team: who of the
// caller's domains is online.
type PresenceRoute struct {
	Access   *AccessHelper
	Presence *PresenceService
	Users    *UsersHelper
}

// Heartbeat marks the caller as active; clients ping every 30 seconds and
// when a tab regains focus.
func (route PresenceRoute) Heartbeat(request *http.Request) *http.Response {
	if _, refusal := (RequestBody{}).Value(request); refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if err := route.Presence.Heartbeat(caller.Email, caller.Domains); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"ok": true})
}

// Team answers every user who shares a domain with the caller, online
// members first, each with their presence.
func (route PresenceRoute) Team(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	viewerDomains := []string{}
	for _, domain := range caller.Domains {
		viewerDomains = append(viewerDomains, strings.ToLower(domain))
	}
	users, err := route.Users.List()
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	snapshot, err := route.Presence.SnapshotForDomains(viewerDomains)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	presence := map[string]map[string]any{}
	for _, entry := range snapshot {
		presence[strings.ToLower(TextValue{}.Of(entry["email"]))] = entry
	}
	team := []map[string]any{}
	seen := map[string]bool{}
	for _, user := range users {
		email := strings.ToLower(TextValue{}.Of(user["email"]))
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		if route.sharesDomain(user["domains"], viewerDomains) {
			team = append(team, route.member(user, presence[email], strings.ToLower(caller.Email)))
		}
	}
	sort.SliceStable(team, func(left, right int) bool {
		if team[left]["online"] != team[right]["online"] {
			return team[left]["online"] == true
		}
		return TextOrder{}.Compare(TextValue{}.Of(team[left]["email"]), TextValue{}.Of(team[right]["email"])) < 0
	})
	return Answer{}.Ok(team)
}

func (PresenceRoute) member(user map[string]any, presence map[string]any, viewer string) map[string]any {
	values := RecordValues{}
	return map[string]any{
		"email":      user["email"],
		"name":       values.Or(user["name"], ""),
		"role":       values.Or(user["role"], ""),
		"domains":    values.Or(user["domains"], []any{}),
		"online":     values.Truthy(presence["online"]),
		"lastSeenAt": values.Or(presence["lastSeenAt"], nil),
		"isSelf":     strings.ToLower(TextValue{}.Of(user["email"])) == viewer,
	}
}

func (PresenceRoute) sharesDomain(value any, viewerDomains []string) bool {
	domains, _ := RecordValues{}.List(value)
	for _, domain := range domains {
		if (PreferenceChoices{}).Includes(viewerDomains, strings.ToLower(TextValue{}.Of(domain))) {
			return true
		}
	}
	return false
}
