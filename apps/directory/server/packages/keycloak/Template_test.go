package keycloak

import (
	"encoding/json"
	"strings"
	"testing"
)

type filledRealm struct {
	Realm              string `json:"realm"`
	DisplayName        string `json:"displayName"`
	EventsEnabled      *bool  `json:"eventsEnabled"`
	AdminEventsEnabled *bool  `json:"adminEventsEnabled"`
	DefaultRole        struct {
		Name string `json:"name"`
	} `json:"defaultRole"`
	Clients []struct {
		RootUrl      string   `json:"rootUrl"`
		RedirectUris []string `json:"redirectUris"`
		Secret       *string  `json:"secret"`
	} `json:"clients"`
	Users []struct {
		RealmRoles []string `json:"realmRoles"`
	} `json:"users"`
}

func fill(t *testing.T, realm string, publicUrl string) (filledRealm, string) {
	content, err := Template{}.Realm(realm, publicUrl)
	if err != nil {
		t.Fatal(err)
	}
	var filled filledRealm
	err = json.Unmarshal(content, &filled)
	if err != nil {
		t.Fatalf("the filled template is not JSON: %v", err)
	}
	return filled, string(content)
}

func TestTemplateFillsTheRealmName(t *testing.T) {
	filled, _ := fill(t, "example.com", "https://mail.example.com")
	if filled.Realm != "example.com" || filled.DisplayName != "example.com" {
		t.Fatalf("got %q, %q", filled.Realm, filled.DisplayName)
	}
	if filled.DefaultRole.Name != "default-roles-example.com" || filled.Users[0].RealmRoles[0] != "default-roles-example.com" {
		t.Fatalf("got %q, %v", filled.DefaultRole.Name, filled.Users[0].RealmRoles)
	}
}

func TestTemplateFillsThePublicUrl(t *testing.T) {
	filled, _ := fill(t, "example.com", "https://mail.example.com")
	if filled.Clients[0].RootUrl != "https://mail.example.com" || filled.Clients[0].RedirectUris[0] != "https://mail.example.com/*" {
		t.Fatalf("got %q, %v", filled.Clients[0].RootUrl, filled.Clients[0].RedirectUris)
	}
}

func TestTemplateLeavesNoPlaceholderOrOldName(t *testing.T) {
	_, content := fill(t, "example.com", "https://mail.example.com")
	if strings.Contains(content, "${") || strings.Contains(content, "default-roles-mesail") || strings.Contains(content, "mesail-workspace-admin") {
		t.Fatalf("got %s", content)
	}
}

func TestTemplateTurnsEventsOffAndSetsNoClientSecret(t *testing.T) {
	filled, _ := fill(t, "example.com", "https://mail.example.com")
	if filled.EventsEnabled == nil || *filled.EventsEnabled || filled.AdminEventsEnabled == nil || *filled.AdminEventsEnabled {
		t.Fatal("events must be set to false")
	}
	if filled.Clients[1].Secret != nil || filled.Clients[2].Secret != nil {
		t.Fatal("the template must not set a client secret")
	}
}

func TestTemplateEscapesValuesForJson(t *testing.T) {
	filled, _ := fill(t, `odd"name\`, `https://mail.example.com/"quoted"`)
	if filled.Realm != `odd"name\` || filled.Clients[0].RootUrl != `https://mail.example.com/"quoted"` {
		t.Fatalf("got %q, %q", filled.Realm, filled.Clients[0].RootUrl)
	}
}
