import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// VCard reads and writes bounded, RFC 6350-compatible vCard files.
type VCard struct{}

// Parse answers the contacts of a vCard file: every card with a name or an
// email address.
func (card VCard) Parse(input string) ([]map[string]any, error) {
	if len(input) > card.largestFile() {
		return nil, errors.New("vCard file exceeds 5MB")
	}
	cards := regexp.MustCompile(`(?is)BEGIN:VCARD.*?END:VCARD`).FindAllString(input, -1)
	if len(cards) > card.mostCards() {
		return nil, errors.New("vCard file contains more than 1000 contacts")
	}
	contacts := []map[string]any{}
	for _, source := range cards {
		if contact := card.parsed(source); contact != nil {
			contacts = append(contacts, contact)
		}
	}
	return contacts, nil
}

// Serialized answers contacts as one vCard file.
func (card VCard) Serialized(contacts []map[string]any) string {
	serialized := []string{}
	for _, contact := range contacts {
		serialized = append(serialized, card.serialized(contact))
	}
	return strings.Join(serialized, "\r\n")
}

func (card VCard) parsed(source string) map[string]any {
	lines := card.unfolded(source)
	properties := []VCardProperty{}
	if len(lines) > 2 {
		for _, line := range lines[1 : len(lines)-1] {
			if property, ok := card.property(line); ok {
				properties = append(properties, property)
			}
		}
	}
	named := func(name string) []VCardProperty {
		found := []VCardProperty{}
		for _, property := range properties {
			if property.Name == name {
				found = append(found, property)
			}
		}
		return found
	}
	first := func(name string) string {
		if found := named(name); len(found) > 0 {
			return found[0].Value
		}
		return ""
	}
	at := func(parts []string, index int) string {
		if index < len(parts) {
			return parts[index]
		}
		return ""
	}
	structured := []string{}
	for _, part := range strings.Split(first("N"), ";") {
		structured = append(structured, card.unescaped(part))
	}
	name := card.unescaped(first("FN"))
	if name == "" {
		joined := []string{}
		for _, part := range []string{at(structured, 1), at(structured, 0)} {
			if part != "" {
				joined = append(joined, part)
			}
		}
		name = strings.Join(joined, " ")
	}
	emails := card.typed(named("EMAIL"), func(value string) string { return strings.ToLower(Security{}.Trim(value)) })
	if name == "" && len(emails) == 0 {
		return nil
	}
	addresses := []any{}
	for _, property := range named("ADR") {
		parts := []string{}
		for _, part := range strings.Split(property.Value, ";") {
			parts = append(parts, card.unescaped(part))
		}
		address := map[string]any{
			"extendedAddress": at(parts, 1), "streetAddress": at(parts, 2), "city": at(parts, 3),
			"region": at(parts, 4), "postalCode": at(parts, 5), "country": at(parts, 6),
		}
		if property.Type != "" {
			address["type"] = property.Type
		}
		addresses = append(addresses, address)
	}
	nicknames := []any{}
	for _, property := range named("NICKNAME") {
		for _, part := range strings.Split(property.Value, ",") {
			if nickname := card.unescaped(part); nickname != "" {
				nicknames = append(nicknames, nickname)
			}
		}
	}
	return map[string]any{
		"name":       name,
		"givenName":  at(structured, 1),
		"familyName": at(structured, 0),
		"company":    card.unescaped(strings.Split(first("ORG"), ";")[0]),
		"title":      card.unescaped(first("TITLE")),
		"emails":     emails,
		"phones":     card.typed(named("TEL"), card.unescaped),
		"addresses":  addresses,
		"urls":       card.typed(named("URL"), card.unescaped),
		"nicknames":  nicknames,
		"birthday":   card.unescaped(first("BDAY")),
		"notes":      card.unescaped(first("NOTE")),
	}
}

func (card VCard) serialized(contact map[string]any) string {
	values := RecordValues{}
	text := func(field string) string { return values.String(values.Or(contact[field], "")) }
	emails, _ := values.List(contact["emails"])
	display := text("name")
	if display == "" {
		parts := []string{}
		for _, field := range []string{"givenName", "familyName"} {
			if part := text(field); part != "" {
				parts = append(parts, part)
			}
		}
		display = strings.Join(parts, " ")
	}
	if display == "" && len(emails) > 0 {
		email, _ := values.Object(emails[0])
		display = values.String(values.Or(email["value"], ""))
	}
	if display == "" {
		display = "Contact"
	}
	lines := []string{"BEGIN:VCARD", "VERSION:4.0", "UID:" + card.escaped(contact["id"]), "FN:" + card.escaped(display)}
	lines = append(lines, "N:"+card.escaped(contact["familyName"])+";"+card.escaped(contact["givenName"])+";;;")
	if values.Truthy(contact["company"]) {
		lines = append(lines, "ORG:"+card.escaped(contact["company"]))
	}
	if values.Truthy(contact["title"]) {
		lines = append(lines, "TITLE:"+card.escaped(contact["title"]))
	}
	typedLines := func(property string, field string) {
		items, _ := values.List(contact[field])
		for _, item := range items {
			entry, _ := values.Object(item)
			lines = append(lines, property+card.typeParameter(entry["type"])+":"+card.escaped(entry["value"]))
		}
	}
	typedLines("EMAIL", "emails")
	typedLines("TEL", "phones")
	addresses, _ := values.List(contact["addresses"])
	for _, item := range addresses {
		address, _ := values.Object(item)
		parts := []string{""}
		for _, field := range []string{"extendedAddress", "streetAddress", "city", "region", "postalCode", "country"} {
			parts = append(parts, card.escaped(address[field]))
		}
		lines = append(lines, "ADR"+card.typeParameter(address["type"])+":"+strings.Join(parts, ";"))
	}
	typedLines("URL", "urls")
	if nicknames, _ := values.List(contact["nicknames"]); len(nicknames) > 0 {
		escaped := []string{}
		for _, nickname := range nicknames {
			escaped = append(escaped, card.escaped(nickname))
		}
		lines = append(lines, "NICKNAME:"+strings.Join(escaped, ","))
	}
	if values.Truthy(contact["birthday"]) {
		lines = append(lines, "BDAY:"+card.escaped(contact["birthday"]))
	}
	if values.Truthy(contact["notes"]) {
		lines = append(lines, "NOTE:"+card.escaped(contact["notes"]))
	}
	lines = append(lines, "END:VCARD")
	folded := []string{}
	for _, line := range lines {
		folded = append(folded, card.folded(line))
	}
	return strings.Join(folded, "\r\n")
}

// unfolded answers a card's logical lines, trailing white space trimmed.
func (VCard) unfolded(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = regexp.MustCompile(`\n[ \t]`).ReplaceAllString(value, "")
	lines := []string{}
	for _, line := range strings.Split(value, "\n") {
		lines = append(lines, strings.TrimRightFunc(line, func(character rune) bool {
			return character == 0xFEFF || (character != '\u0085' && unicode.IsSpace(character))
		}))
	}
	return lines
}

// property splits one content line into its name (group prefix dropped),
// its value and the first TYPE parameter.
func (VCard) property(line string) (VCardProperty, bool) {
	colon := -1
	quoted := false
	for index := 0; index < len(line); index++ {
		if line[index] == '"' {
			quoted = !quoted
		}
		if line[index] == ':' && !quoted {
			colon = index
			break
		}
	}
	if colon <= 0 {
		return VCardProperty{}, false
	}
	head := strings.Split(line[:colon], ";")
	groups := strings.Split(head[0], ".")
	parameters := map[string]string{}
	for _, parameter := range head[1:] {
		key, rest, _ := strings.Cut(parameter, "=")
		parameters[strings.ToUpper(key)] = regexp.MustCompile(`^"|"$`).ReplaceAllString(rest, "")
	}
	kind := strings.ToLower(strings.Split(parameters["TYPE"], ",")[0])
	return VCardProperty{Name: strings.ToUpper(groups[len(groups)-1]), Value: line[colon+1:], Type: kind}, true
}

func (VCard) typed(properties []VCardProperty, transform func(string) string) []any {
	typed := []any{}
	for _, property := range properties {
		item := map[string]any{"value": transform(property.Value)}
		if property.Type != "" {
			item["type"] = property.Type
		}
		if item["value"] != "" {
			typed = append(typed, item)
		}
	}
	return typed
}

func (VCard) unescaped(value string) string {
	value = regexp.MustCompile(`(?i)\\n`).ReplaceAllString(value, "\n")
	value = strings.ReplaceAll(value, `\,`, ",")
	value = strings.ReplaceAll(value, `\;`, ";")
	return strings.ReplaceAll(value, `\\`, `\`)
}

func (VCard) escaped(value any) string {
	text := RecordValues{}.String(RecordValues{}.Or(value, ""))
	text = strings.ReplaceAll(text, `\`, `\\`)
	text = regexp.MustCompile(`\r?\n`).ReplaceAllString(text, `\n`)
	text = strings.ReplaceAll(text, ",", `\,`)
	return strings.ReplaceAll(text, ";", `\;`)
}

func (VCard) typeParameter(value any) string {
	safe := regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(strings.ToLower(RecordValues{}.String(RecordValues{}.Or(value, ""))), "")
	if safe == "" {
		return ""
	}
	return ";TYPE=" + safe
}

// folded folds a line so no physical line passes 73 octets.
func (VCard) folded(line string) string {
	chunks := []string{}
	current := ""
	for len(line) > 0 {
		_, width := utf8.DecodeRuneInString(line)
		character := line[:width]
		line = line[width:]
		if len(current)+len(character) > 73 {
			chunks = append(chunks, current)
			current = character
		} else {
			current += character
		}
	}
	return strings.Join(append(chunks, current), "\r\n ")
}

func (VCard) largestFile() int { return 5 * 1024 * 1024 }

func (VCard) mostCards() int { return 1000 }
