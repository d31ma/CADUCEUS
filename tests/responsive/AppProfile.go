//go:build responsive

package responsive

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

type AppProfile struct {
	Name              string
	DefaultDist       string
	SessionStorageKey string
	StoredSession     any
	HostedDomains     []string
	DomainRoles       map[string][]string
	DefaultRoles      []string
	DataAnswers       map[string]any
}

type fakeConversationPage struct {
	Items      []any  `json:"items"`
	NextCursor string `json:"nextCursor"`
}

type fakeMemberPage struct {
	Items []any `json:"items"`
}

const (
	webAppName        = "web"
	adminAppName      = "web-admin"
	memberRole        = "mesail-member"
	adminRole         = "mesail-admin"
	operatorRole      = "mesail-operator"
	operatorRealm     = "mesail-operators"
	firstHostedDomain = "example.com"
	otherHostedDomain = "example.org"
)

func AppProfiles() []AppProfile {
	return []AppProfile{WebProfile(), AdminProfile()}
}

func WebProfile() AppProfile {
	return AppProfile{
		Name:              webAppName,
		DefaultDist:       "../../apps/web/dist/web",
		SessionStorageKey: "tac:service:session:/:sessionKey",
		StoredSession:     sessionKeyPrefix + firstHostedDomain,
		HostedDomains:     []string{firstHostedDomain, otherHostedDomain},
		DomainRoles:       map[string][]string{},
		DefaultRoles:      []string{memberRole},
		DataAnswers: map[string]any{
			"GET /api/v1/channels":                    []any{},
			"GET /api/v1/messaging/conversations":     fakeConversationPage{Items: []any{}},
			"PUT /api/v1/realtime/presence":           nil,
			"PUT /api/v1/directory/me/default-domain": nil,
		},
	}
}

func AdminProfile() AppProfile {
	return AppProfile{
		Name:              adminAppName,
		DefaultDist:       "../../apps/web-admin/dist/web",
		SessionStorageKey: "tac:service:adminSession:/:sessionKey",
		StoredSession:     sessionKeyPrefix + firstHostedDomain,
		HostedDomains:     []string{firstHostedDomain, otherHostedDomain, operatorRealm},
		DomainRoles: map[string][]string{
			firstHostedDomain: {adminRole, memberRole},
			operatorRealm:     {operatorRole},
		},
		DefaultRoles: []string{memberRole},
		DataAnswers: map[string]any{
			"GET /api/v1/directory/domains": []any{},
			"GET /api/v1/directory/members": fakeMemberPage{Items: []any{}},
			"GET /api/v1/channels":          []any{},
		},
	}
}

func SelectAppProfile(name string) (AppProfile, error) {
	wanted := strings.TrimSpace(name)
	if wanted == "" {
		return WebProfile(), nil
	}
	for _, profile := range AppProfiles() {
		if profile.Name == wanted {
			return profile, nil
		}
	}
	return AppProfile{}, fmt.Errorf("MESAIL_APP=%s is not a known app; use one of: %s", name, strings.Join(appNames(), ", "))
}

func appNames() []string {
	names := []string{}
	for _, profile := range AppProfiles() {
		names = append(names, profile.Name)
	}
	return names
}

func (profile AppProfile) StoredSessionText() string {
	encoded, _ := json.Marshal(profile.StoredSession)
	return string(encoded)
}

func (profile AppProfile) Hosts(domain string) bool {
	return slices.Contains(profile.HostedDomains, domain)
}

func (profile AppProfile) RolesFor(domain string) []string {
	roles, found := profile.DomainRoles[domain]
	if !found {
		return profile.DefaultRoles
	}
	return roles
}

func (profile AppProfile) DataAnswer(request *http.Request) (any, bool) {
	answer, found := profile.DataAnswers[request.Method+" "+strings.TrimSuffix(request.URL.Path, "/")]
	return answer, found
}

func (profile AppProfile) BuildCommand() string {
	return "ty bundle apps/" + profile.Name + " --target web"
}
