// Package contactapi contains the bounded provider-neutral contact encoding
// shared by native JSON contact APIs before exact-owner FYLO persistence.
package contactapi

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/d31ma/caduceus/internal/contactsync"
)

var ErrInvalidContact = errors.New("provider contact is invalid")

// RenderVCard creates the RFC 6350 representation retained behind the trusted
// provider boundary. Provider-native identifiers are carried as UID values.
func RenderVCard(contact contactsync.Contact) (string, error) {
	if strings.TrimSpace(contact.UID) == "" || invalidDesired(contact) {
		return "", ErrInvalidContact
	}
	name := strings.TrimSpace(contact.Name)
	if name == "" {
		name = strings.TrimSpace(strings.Join([]string{contact.GivenName, contact.FamilyName}, " "))
	}
	if name == "" && len(contact.Emails) > 0 {
		name = contact.Emails[0].Value
	}
	if name == "" && len(contact.Phones) > 0 {
		name = contact.Phones[0].Value
	}
	if name == "" {
		return "", ErrInvalidContact
	}
	lines := []string{
		"BEGIN:VCARD", "VERSION:4.0", "UID:" + escape(contact.UID),
		"N:" + escape(contact.FamilyName) + ";" + escape(contact.GivenName) + ";;;", "FN:" + escape(name),
	}
	appendText := func(property, value string) {
		if strings.TrimSpace(value) != "" {
			lines = append(lines, property+":"+escape(value))
		}
	}
	appendText("ORG", contact.Company)
	appendText("TITLE", contact.Title)
	for _, value := range contact.Emails {
		lines = append(lines, "EMAIL"+typeParameter(value.Type)+":"+escape(value.Value))
	}
	for _, value := range contact.Phones {
		lines = append(lines, "TEL"+typeParameter(value.Type)+":"+escape(value.Value))
	}
	for _, address := range contact.Addresses {
		lines = append(lines, "ADR"+typeParameter(address.Type)+":;"+escape(address.ExtendedAddress)+";"+escape(address.StreetAddress)+";"+escape(address.City)+";"+escape(address.Region)+";"+escape(address.PostalCode)+";"+escape(address.Country))
	}
	for _, value := range contact.URLs {
		lines = append(lines, "URL"+typeParameter(value.Type)+":"+escape(value.Value))
	}
	if len(contact.Nicknames) > 0 {
		values := make([]string, 0, len(contact.Nicknames))
		for _, nickname := range contact.Nicknames {
			values = append(values, escape(nickname))
		}
		lines = append(lines, "NICKNAME:"+strings.Join(values, ","))
	}
	appendText("BDAY", contact.Birthday)
	appendText("NOTE", contact.Notes)
	lines = append(lines, "END:VCARD")
	for index, line := range lines {
		lines[index] = fold(line)
	}
	return strings.Join(lines, "\r\n") + "\r\n", nil
}

func invalidDesired(contact contactsync.Contact) bool {
	contact.ID = ""
	contact.AccountID = ""
	contact.AddressBookID = ""
	contact.Href = ""
	contact.ETag = ""
	contact.RawVCard = ""
	contact.RemoteUpdatedAt = time.Time{}
	request := contactsync.MutationRequest{
		ID: "cmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "provider-projection", Kind: contactsync.MutationUpdate,
		ContactID: "carddav_abcdefghijklmnopqrstuvwx", ExpectedETag: "projection", Desired: contact,
	}
	return contactsync.ValidateMutation(request) != nil
}

func typeParameter(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var safe strings.Builder
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' {
			safe.WriteRune(character)
		}
	}
	if safe.Len() == 0 {
		return ""
	}
	return ";TYPE=" + safe.String()
}

func escape(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\r\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\n")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, ";", "\\;")
	return strings.ReplaceAll(value, ",", "\\,")
}

func fold(line string) string {
	if len(line) <= 75 {
		return line
	}
	var result strings.Builder
	width := 0
	for len(line) > 0 {
		character, size := utf8.DecodeRuneInString(line)
		if width > 0 && width+size > 75 {
			result.WriteString("\r\n ")
			width = 1
		}
		result.WriteRune(character)
		width += size
		line = line[size:]
	}
	return result.String()
}
