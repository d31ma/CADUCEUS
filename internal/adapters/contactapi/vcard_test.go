package contactapi

import (
	"strings"
	"testing"

	"github.com/d31ma/caduceus/internal/contactsync"
)

func TestRenderVCardProjectsStructuredProviderContact(t *testing.T) {
	contact := contactsync.Contact{UID: "people/123", Name: strings.Repeat("Résumé ", 20), GivenName: "Alice", FamilyName: "Example", Company: "Example", Title: "Director",
		Emails: []contactsync.Value{{Value: "alice@example.com", Type: "work"}}, Phones: []contactsync.Value{{Value: "+15551234567", Type: "mobile"}},
		Addresses: []contactsync.Address{{StreetAddress: "1 Main St", City: "Toronto", Region: "ON", PostalCode: "M5V 1A1", Country: "Canada", Type: "work"}}, Notes: "line one\nline two"}
	raw, err := RenderVCard(contact)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"VERSION:4.0", "UID:people/123", "EMAIL;TYPE=work:alice@example.com", "ADR;TYPE=work:", "NOTE:line one\\nline two"} {
		if !strings.Contains(raw, required) {
			t.Fatalf("vCard missing %q:\n%s", required, raw)
		}
	}
	for _, line := range strings.Split(strings.TrimSuffix(raw, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Fatalf("line has %d octets: %q", len(line), line)
		}
	}
}

func TestRenderVCardRejectsMissingIdentityAndControlInjection(t *testing.T) {
	for _, contact := range []contactsync.Contact{{Name: "No identity"}, {UID: "people/1", Name: "Alice\r\nX-INJECTED:true"}} {
		if _, err := RenderVCard(contact); err == nil {
			t.Fatalf("rendered invalid contact %#v", contact)
		}
	}
}
