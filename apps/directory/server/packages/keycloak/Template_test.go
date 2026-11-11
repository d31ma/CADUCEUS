package keycloak

// tachyon: example secrets

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

type filledRealm struct {
	Realm   string `json:"realm"`
	Clients []struct {
		ClientId string `json:"clientId"`
		Secret   string `json:"secret"`
	} `json:"clients"`
}

func exampleSecrets() RealmSecrets {
	return RealmSecrets{Identity: "example-identity", Service: "example-service", Account: "example-account"}
}

func fill(t *testing.T, realm string, secrets RealmSecrets) (filledRealm, string) {
	content, err := Template{}.Realm(realm, secrets)
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

func secretOf(filled filledRealm, clientId string) string {
	for _, client := range filled.Clients {
		if client.ClientId == clientId {
			return client.Secret
		}
	}
	return ""
}

func TestTemplateFillsTheRealmName(t *testing.T) {
	filled, content := fill(t, "example.com", exampleSecrets())
	if filled.Realm != "example.com" || !strings.Contains(content, "default-roles-example.com") {
		t.Fatalf("got %q", filled.Realm)
	}
}

func TestTemplateFillsTheClientSecrets(t *testing.T) {
	filled, _ := fill(t, "example.com", exampleSecrets())
	if secretOf(filled, "mesail-identity") != "example-identity" || secretOf(filled, "mesail-service") != "example-service" {
		t.Fatalf("got %+v", filled.Clients)
	}
}

func TestTemplateLeavesNoPlaceholder(t *testing.T) {
	_, content := fill(t, "example.com", exampleSecrets())
	if strings.Contains(content, "${") {
		t.Fatalf("got %s", content)
	}
}

func TestTemplateRefusesAnEmptySecret(t *testing.T) {
	_, err := Template{}.Realm("example.com", RealmSecrets{Identity: "example-identity", Service: "example-service"})
	if !errors.As(err, &NotConfigured{}) {
		t.Fatalf("got %v", err)
	}
}

func TestTemplateEscapesValuesForJson(t *testing.T) {
	filled, _ := fill(t, `odd"name\`, exampleSecrets())
	if filled.Realm != `odd"name\` {
		t.Fatalf("got %q", filled.Realm)
	}
}

func TestTemplateIsTheKeycloakDomainRealm(t *testing.T) {
	source, err := os.ReadFile("../../../../../deploy/keycloak/realm/domain-realm.json")
	if err != nil || !bytes.Equal(source, realmTemplate) {
		t.Fatalf("realm-template.json differs from deploy/keycloak/realm/domain-realm.json (%v); copy it again", err)
	}
}
