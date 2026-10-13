package carddav

import (
	"bufio"
	"errors"
	"io"
	"mime/quotedprintable"
	"strings"
	"unicode/utf8"

	"github.com/d31ma/caduceus/internal/contactsync"
)

func parseVCard(raw string) (contactsync.Contact, error) {
	if len(raw) == 0 || len(raw) > contactsync.MaxVCardBytes {
		return contactsync.Contact{}, errors.New("invalid vCard")
	}
	lines := unfoldVCard(raw)
	if len(lines) < 2 || !strings.EqualFold(strings.TrimSpace(lines[0]), "BEGIN:VCARD") {
		return contactsync.Contact{}, errors.New("invalid vCard")
	}
	contact := contactsync.Contact{}
	ended := false
	for _, line := range lines[1:] {
		if strings.EqualFold(strings.TrimSpace(line), "END:VCARD") {
			ended = true
			break
		}
		left, encoded, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		parts := strings.Split(left, ";")
		name := canonicalPropertyName(parts[0])
		parameters := parseParameters(parts[1:])
		value := decodeVCardValue(encoded, parameters)
		value = unwrapURIValue(name, value, parameters)
		typeName := firstType(parameters["TYPE"])
		switch name {
		case "UID":
			contact.UID = bounded(value, 512)
		case "FN":
			contact.Name = bounded(value, 512)
		case "N":
			fields := splitEscaped(value, ';')
			if len(fields) > 0 {
				contact.FamilyName = bounded(fields[0], 256)
			}
			if len(fields) > 1 {
				contact.GivenName = bounded(fields[1], 256)
			}
		case "ORG":
			contact.Company = bounded(firstSplit(value, ';'), 512)
		case "TITLE":
			contact.Title = bounded(value, 512)
		case "EMAIL":
			if normalized := bounded(strings.TrimSpace(value), 512); normalized != "" && len(contact.Emails) < 100 {
				contact.Emails = append(contact.Emails, contactsync.Value{Value: normalized, Type: typeName})
			}
		case "TEL":
			if normalized := bounded(strings.TrimSpace(value), 256); normalized != "" && len(contact.Phones) < 100 {
				contact.Phones = append(contact.Phones, contactsync.Value{Value: normalized, Type: typeName})
			}
		case "ADR":
			fields := splitEscaped(value, ';')
			for len(fields) < 7 {
				fields = append(fields, "")
			}
			if len(contact.Addresses) < 100 {
				contact.Addresses = append(contact.Addresses, contactsync.Address{ExtendedAddress: bounded(fields[1], 512), StreetAddress: bounded(fields[2], 512), City: bounded(fields[3], 256), Region: bounded(fields[4], 256), PostalCode: bounded(fields[5], 128), Country: bounded(fields[6], 256), Type: typeName})
			}
		case "URL":
			if normalized := bounded(strings.TrimSpace(value), 2048); normalized != "" && len(contact.URLs) < 100 {
				contact.URLs = append(contact.URLs, contactsync.Value{Value: normalized, Type: typeName})
			}
		case "NICKNAME":
			for _, nickname := range splitEscaped(value, ',') {
				if len(contact.Nicknames) < 100 && strings.TrimSpace(nickname) != "" {
					contact.Nicknames = append(contact.Nicknames, bounded(nickname, 256))
				}
			}
		case "BDAY":
			contact.Birthday = bounded(value, 64)
		case "NOTE":
			contact.Notes = bounded(value, 1<<16)
		}
	}
	if !ended {
		return contactsync.Contact{}, errors.New("invalid vCard")
	}
	if contact.Name == "" {
		contact.Name = strings.TrimSpace(strings.Join([]string{contact.GivenName, contact.FamilyName}, " "))
	}
	if contact.Name == "" && len(contact.Emails) > 0 {
		contact.Name = contact.Emails[0].Value
	}
	if contact.Name == "" && len(contact.Phones) > 0 {
		contact.Name = contact.Phones[0].Value
	}
	if contact.Name == "" {
		return contactsync.Contact{}, errors.New("invalid vCard")
	}
	return contact, nil
}

var managedVCardProperties = map[string]bool{
	"FN": true, "N": true, "ORG": true, "TITLE": true, "EMAIL": true, "TEL": true,
	"ADR": true, "URL": true, "NICKNAME": true, "BDAY": true, "NOTE": true,
}

// mergeVCard replaces CADUCEUS-managed contact fields while retaining UID,
// PHOTO, cryptographic keys, and vendor extensions it does not understand.
func mergeVCard(raw string, contact contactsync.Contact) (string, error) {
	lines := unfoldVCard(raw)
	if len(lines) < 2 || !strings.EqualFold(lines[0], "BEGIN:VCARD") {
		return "", errors.New("invalid vCard")
	}
	result := []string{"BEGIN:VCARD"}
	hasVersion, hasUID, ended := false, false, false
	for _, line := range lines[1:] {
		if strings.EqualFold(strings.TrimSpace(line), "END:VCARD") {
			ended = true
			break
		}
		left, _, found := strings.Cut(line, ":")
		name := canonicalPropertyName(strings.Split(left, ";")[0])
		if name == "VERSION" {
			if !hasVersion {
				result = append(result, "VERSION:3.0")
				hasVersion = true
			}
			continue
		}
		if name == "UID" {
			hasUID = true
		}
		if found && !managedVCardProperties[name] && name != "BEGIN" {
			result = append(result, line)
		}
	}
	if !ended {
		return "", errors.New("invalid vCard")
	}
	if !hasVersion {
		result = append(result, "VERSION:3.0")
	}
	uid := strings.TrimSpace(contact.UID)
	if !hasUID {
		if uid == "" {
			uid = strings.TrimSpace(contact.ID)
		}
		if uid == "" {
			return "", errors.New("vCard UID is required")
		}
		result = append(result, "UID:"+escapeVCard(uid))
	}
	name := strings.TrimSpace(contact.Name)
	if name == "" {
		name = strings.TrimSpace(strings.Join([]string{contact.GivenName, contact.FamilyName}, " "))
	}
	if name == "" {
		return "", errors.New("vCard display name is required")
	}
	result = append(result, "N:"+escapeVCard(contact.FamilyName)+";"+escapeVCard(contact.GivenName)+";;;", "FN:"+escapeVCard(name))
	appendTextProperty := func(property, value string) {
		if strings.TrimSpace(value) != "" {
			result = append(result, property+":"+escapeVCard(value))
		}
	}
	appendTextProperty("ORG", contact.Company)
	appendTextProperty("TITLE", contact.Title)
	for _, email := range contact.Emails {
		appendValueProperty(&result, "EMAIL", email)
	}
	for _, phone := range contact.Phones {
		appendValueProperty(&result, "TEL", phone)
	}
	for _, address := range contact.Addresses {
		property := "ADR" + typeParameter(address.Type)
		result = append(result, property+":;"+escapeVCard(address.ExtendedAddress)+";"+escapeVCard(address.StreetAddress)+";"+escapeVCard(address.City)+";"+escapeVCard(address.Region)+";"+escapeVCard(address.PostalCode)+";"+escapeVCard(address.Country))
	}
	for _, website := range contact.URLs {
		appendValueProperty(&result, "URL", website)
	}
	if len(contact.Nicknames) > 0 {
		values := make([]string, 0, len(contact.Nicknames))
		for _, nickname := range contact.Nicknames {
			values = append(values, escapeVCard(nickname))
		}
		result = append(result, "NICKNAME:"+strings.Join(values, ","))
	}
	appendTextProperty("BDAY", contact.Birthday)
	appendTextProperty("NOTE", contact.Notes)
	result = append(result, "END:VCARD")
	for index, line := range result {
		result[index] = foldVCardLine(line)
	}
	return strings.Join(result, "\r\n") + "\r\n", nil
}

func canonicalPropertyName(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if separator := strings.LastIndexByte(value, '.'); separator >= 0 {
		value = value[separator+1:]
	}
	return value
}

func unwrapURIValue(property, value string, parameters map[string][]string) string {
	if !contains(parameters["VALUE"], "uri") {
		return value
	}
	switch property {
	case "EMAIL":
		return trimURIScheme(value, "mailto:")
	case "TEL":
		return trimURIScheme(value, "tel:")
	default:
		return value
	}
}

func trimURIScheme(value, scheme string) string {
	if len(value) >= len(scheme) && strings.EqualFold(value[:len(scheme)], scheme) {
		return value[len(scheme):]
	}
	return value
}

func appendValueProperty(lines *[]string, property string, value contactsync.Value) {
	if strings.TrimSpace(value.Value) != "" {
		*lines = append(*lines, property+typeParameter(value.Type)+":"+escapeVCard(value.Value))
	}
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

func escapeVCard(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\r\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\n")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, ";", "\\;")
	return strings.ReplaceAll(value, ",", "\\,")
}

func foldVCardLine(line string) string {
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

func unfoldVCard(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	physical := strings.Split(raw, "\n")
	logical := make([]string, 0, len(physical))
	for _, line := range physical {
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') && len(logical) > 0 {
			logical[len(logical)-1] += line[1:]
			continue
		}
		if line != "" {
			logical = append(logical, line)
		}
	}
	return logical
}

func parseParameters(parts []string) map[string][]string {
	result := make(map[string][]string)
	for _, part := range parts {
		key, value, found := strings.Cut(part, "=")
		if !found {
			result["TYPE"] = append(result["TYPE"], strings.ToLower(strings.TrimSpace(key)))
			continue
		}
		key = strings.ToUpper(strings.TrimSpace(key))
		for _, item := range strings.Split(strings.Trim(value, `"`), ",") {
			if item = strings.TrimSpace(item); item != "" {
				result[key] = append(result[key], strings.ToLower(item))
			}
		}
	}
	return result
}

func decodeVCardValue(value string, parameters map[string][]string) string {
	if contains(parameters["ENCODING"], "quoted-printable") {
		decoded, err := io.ReadAll(quotedprintable.NewReader(bufio.NewReader(strings.NewReader(value))))
		if err == nil {
			value = string(decoded)
		}
	}
	return unescapeVCard(value)
}

func unescapeVCard(value string) string {
	var result strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 >= len(value) {
			result.WriteByte(value[index])
			continue
		}
		index++
		switch value[index] {
		case 'n', 'N':
			result.WriteByte('\n')
		case '\\', ',', ';':
			result.WriteByte(value[index])
		default:
			result.WriteByte(value[index])
		}
	}
	return strings.TrimSpace(result.String())
}

func splitEscaped(value string, separator byte) []string {
	var result []string
	var current strings.Builder
	escaped := false
	for index := 0; index < len(value); index++ {
		character := value[index]
		if escaped {
			current.WriteByte(character)
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			current.WriteByte(character)
			continue
		}
		if character == separator {
			result = append(result, unescapeVCard(current.String()))
			current.Reset()
			continue
		}
		current.WriteByte(character)
	}
	result = append(result, unescapeVCard(current.String()))
	return result
}

func firstType(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return bounded(values[0], 40)
}

func firstSplit(value string, separator byte) string {
	values := splitEscaped(value, separator)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}

func bounded(value string, limit int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\x00", ""))
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
