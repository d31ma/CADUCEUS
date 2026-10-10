package carddav

import (
	"strings"
	"testing"

	"github.com/d31ma/caduceus/internal/contactsync"
)

func TestParseVCardProjectsContactFieldsAndUnfoldsLines(t *testing.T) {
	contact, err := parseVCard("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:remote-7\r\nN:Example;Alice;;;\r\nFN:Alice Example\r\nORG:Example Corp;Mail\r\nTITLE:Director\r\nEMAIL;TYPE=WORK:alice@example.com\r\nTEL;TYPE=CELL:+15551234567\r\nADR;TYPE=work:;Suite 7;1 Main St;Toronto;ON;M5V 1A1;Canada\r\nURL:https://example.com/alice\r\nNICKNAME:Al,Ali\r\nNOTE:First line\\nsecond line that is \r\n folded\r\nEND:VCARD\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if contact.UID != "remote-7" || contact.Name != "Alice Example" || contact.GivenName != "Alice" || contact.FamilyName != "Example" || contact.Company != "Example Corp" || contact.Title != "Director" {
		t.Fatalf("identity fields = %#v", contact)
	}
	if len(contact.Emails) != 1 || contact.Emails[0].Type != "work" || len(contact.Phones) != 1 || contact.Phones[0].Type != "cell" {
		t.Fatalf("values = %#v %#v", contact.Emails, contact.Phones)
	}
	if len(contact.Addresses) != 1 || contact.Addresses[0].StreetAddress != "1 Main St" || contact.Addresses[0].City != "Toronto" || len(contact.Nicknames) != 2 || contact.Notes != "First line\nsecond line that is folded" {
		t.Fatalf("details = %#v nicknames=%#v notes=%q", contact.Addresses, contact.Nicknames, contact.Notes)
	}
}

func TestMergeVCardPreservesUnknownPropertiesAndReplacesManagedFields(t *testing.T) {
	raw := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:remote-7\r\nFN:Old Name\r\nEMAIL:old@example.com\r\nPHOTO;VALUE=uri:https://example.com/photo.jpg\r\nX-CUSTOM:retain-me\r\nEND:VCARD\r\n"
	updated, err := mergeVCard(raw, contactsync.Contact{ID: "carddav_7", Name: "Alice Updated", GivenName: "Alice", FamilyName: "Updated", Emails: []contactsync.Value{{Value: "alice@example.com", Type: "work"}}, Notes: "line one\nline two"})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"VERSION:3.0", "UID:remote-7", "FN:Alice Updated", "EMAIL;TYPE=work:alice@example.com", "PHOTO;VALUE=uri:https://example.com/photo.jpg", "X-CUSTOM:retain-me", "NOTE:line one\\nline two"} {
		if !strings.Contains(updated, expected) {
			t.Fatalf("updated vCard missing %q:\n%s", expected, updated)
		}
	}
	if strings.Contains(updated, "Old Name") || strings.Contains(updated, "old@example.com") {
		t.Fatalf("stale fields retained:\n%s", updated)
	}
}

func TestVCardInteroperabilityHandlesAppleGroupsAndVersionFourURIValues(t *testing.T) {
	raw := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:apple-7\r\nFN:Alice Example\r\nitem1.EMAIL;TYPE=INTERNET;TYPE=pref;VALUE=uri:MAILTO:alice@example.com\r\nitem2.TEL;TYPE=CELL;VALUE=uri:TEL:+15551234567\r\nitem1.X-ABLabel:_$!<Work>!$_\r\nX-APPLE-OMIT-YEAR:1604\r\nEND:VCARD\r\n"
	contact, err := parseVCard(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(contact.Emails) != 1 || contact.Emails[0].Value != "alice@example.com" || len(contact.Phones) != 1 || contact.Phones[0].Value != "+15551234567" {
		t.Fatalf("contact=%#v", contact)
	}
	updated, err := mergeVCard(raw, contactsync.Contact{UID: contact.UID, Name: "Alice Updated", Emails: []contactsync.Value{{Value: "new@example.com", Type: "work"}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(updated, "alice@example.com") || strings.Contains(updated, "+15551234567") || !strings.Contains(updated, "EMAIL;TYPE=work:new@example.com") || !strings.Contains(updated, "X-APPLE-OMIT-YEAR:1604") {
		t.Fatalf("updated=%q", updated)
	}
}
