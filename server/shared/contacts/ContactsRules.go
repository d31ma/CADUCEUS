import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf16"
)

// ContactsRules validate and normalize the contacts a mailbox owns, the
// contacts its provider address books hold, and the text search over them.
type ContactsRules struct{}

// Normalized answers a contact of an owner built from input, with the
// identity and timestamps of metadata, or the first problem with it.
func (rules ContactsRules) Normalized(owner string, input map[string]any, metadata map[string]any) (map[string]any, error) {
	emails, err := rules.values(input["emails"], func(value string) (string, error) {
		email := Security{}.NormalizeEmailAddress(value)
		if email == "" {
			return "", errors.New("invalid contact email")
		}
		return email, nil
	})
	if err != nil {
		return nil, err
	}
	phones, err := rules.values(input["phones"], func(value string) (string, error) { return rules.Text(value, "phone", 80, true) })
	if err != nil {
		return nil, err
	}
	urls, err := rules.values(input["urls"], rules.webAddress)
	if err != nil {
		return nil, err
	}
	addresses, err := rules.addresses(input["addresses"])
	if err != nil {
		return nil, err
	}
	name, err := rules.Text(input["name"], "name", 200, false)
	if err != nil {
		return nil, err
	}
	if name == "" && len(emails) == 0 && len(phones) == 0 {
		return nil, errors.New("name, email, or phone is required")
	}
	domain, err := rules.OwnerDomain(owner)
	if err != nil {
		return nil, err
	}
	contact := map[string]any{
		"id": metadata["id"], "ownerEmail": strings.ToLower(owner), "domain": domain, "source": metadata["source"], "name": name,
		"emails": emails, "phones": phones, "addresses": addresses, "urls": urls,
		"createdAt": metadata["createdAt"], "updatedAt": metadata["updatedAt"],
	}
	for _, field := range []struct {
		name string
		max  int
	}{{"givenName", 100}, {"familyName", 100}, {"company", 200}, {"title", 200}} {
		if contact[field.name], err = rules.Text(input[field.name], field.name, field.max, false); err != nil {
			return nil, err
		}
	}
	if contact["nicknames"], err = rules.unique(input["nicknames"], 100, 50); err != nil {
		return nil, err
	}
	if contact["birthday"], err = rules.Text(input["birthday"], "birthday", 32, false); err != nil {
		return nil, err
	}
	if contact["notes"], err = rules.Text(input["notes"], "notes", 10000, false); err != nil {
		return nil, err
	}
	if contact["groupIds"], err = rules.unique(input["groupIds"], 140, 100); err != nil {
		return nil, err
	}
	if count, isNumber := (RecordValues{}).Number(metadata["interactionCount"]); isNumber && count == float64(int64(count)) {
		contact["interactionCount"] = count
	}
	if (RecordValues{}).Truthy(metadata["lastContactedAt"]) {
		contact["lastContactedAt"] = RecordValues{}.String(metadata["lastContactedAt"])
	}
	return contact, nil
}

// ProviderContact answers a CardDAV contact in the owner's contact model.
func (rules ContactsRules) ProviderContact(owner string, input any) map[string]any {
	values := RecordValues{}
	source, _ := values.Object(input)
	contact := StoredRecords{}.Without(source)
	domain, _ := rules.OwnerDomain(owner)
	contact["ownerEmail"], contact["domain"], contact["source"], contact["readOnly"] = strings.ToLower(owner), domain, "carddav", false
	for _, field := range []string{"name", "givenName", "familyName", "company", "title", "birthday", "notes"} {
		contact[field] = values.String(values.Or(source[field], ""))
	}
	for _, field := range []string{"emails", "phones", "addresses", "urls", "nicknames"} {
		list, isList := values.List(source[field])
		if !isList {
			list = []any{}
		}
		contact[field] = list
	}
	contact["groupIds"] = []any{}
	return contact
}

// Text answers trimmed text of at most max UTF-16 units without NUL or CR,
// or the problem with it.
func (rules ContactsRules) Text(value any, field string, max int, required bool) (string, error) {
	text := Security{}.Trim(RecordValues{}.Text(value))
	if required && text == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if rules.Length(text) > max || strings.ContainsAny(text, "\x00\r") {
		return "", fmt.Errorf("%s is invalid", field)
	}
	return text, nil
}

// OwnerDomain answers the lower-case domain of an owner's address.
func (ContactsRules) OwnerDomain(email string) (string, error) {
	parts := strings.Split(strings.ToLower(email), "@")
	if len(parts) < 2 || parts[1] == "" {
		return "", errors.New("invalid contact owner")
	}
	return parts[1], nil
}

// Searchable answers the lower-case text a search matches a contact by.
func (ContactsRules) Searchable(contact map[string]any) string {
	values := RecordValues{}
	parts := []string{}
	add := func(value any) {
		if value == nil {
			parts = append(parts, "")
			return
		}
		parts = append(parts, values.String(value))
	}
	for _, field := range []string{"name", "givenName", "familyName", "company", "title", "notes", "birthday"} {
		add(contact[field])
	}
	nicknames, _ := values.List(contact["nicknames"])
	for _, nickname := range nicknames {
		add(nickname)
	}
	for _, field := range []string{"emails", "phones", "urls"} {
		items, _ := values.List(contact[field])
		for _, item := range items {
			entry, _ := values.Object(item)
			add(entry["value"])
		}
	}
	addresses, _ := values.List(contact["addresses"])
	for _, item := range addresses {
		address, _ := values.Object(item)
		for _, field := range []string{"streetAddress", "extendedAddress", "city", "region", "postalCode", "country"} {
			add(address[field])
		}
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

// Length counts text in UTF-16 code units, as JavaScript's length did.
func (ContactsRules) Length(text string) int {
	return len(utf16.Encode([]rune(text)))
}

// Prefix answers the first count UTF-16 code units of text, as
// String.prototype.slice(0, count) did.
func (ContactsRules) Prefix(text string, count int) string {
	units := utf16.Encode([]rune(text))
	if len(units) <= count {
		return text
	}
	return string(utf16.Decode(units[:count]))
}

// values answers the first 50 distinct {value, type} entries of a list of
// text values or {value, type} objects, each transformed.
func (rules ContactsRules) values(input any, transform func(string) (string, error)) ([]any, error) {
	entries, isList := RecordValues{}.List(input)
	normalized := []any{}
	if !isList {
		return normalized, nil
	}
	if len(entries) > rules.mostValues() {
		entries = entries[:rules.mostValues()]
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		object, isObject := RecordValues{}.Object(entry)
		raw, isText := entry.(string)
		if isObject {
			raw, isText = object["value"].(string)
		}
		if !isText || (Security{}).Trim(raw) == "" {
			continue
		}
		value, err := transform(Security{}.Trim(raw))
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		item := map[string]any{"value": value}
		if isObject && (RecordValues{}).Truthy(object["type"]) {
			kind, err := rules.kind(object["type"])
			if err != nil {
				return nil, err
			}
			if kind != "" {
				item["type"] = kind
			}
		}
		normalized = append(normalized, item)
	}
	return normalized, nil
}

func (rules ContactsRules) addresses(input any) ([]any, error) {
	entries, isList := RecordValues{}.List(input)
	addresses := []any{}
	if !isList {
		return addresses, nil
	}
	if len(entries) > rules.mostValues() {
		entries = entries[:rules.mostValues()]
	}
	for _, entry := range entries {
		source, _ := RecordValues{}.Object(entry)
		address := map[string]any{}
		for _, field := range []struct {
			name string
			max  int
		}{{"streetAddress", 300}, {"extendedAddress", 300}, {"city", 120}, {"region", 120}, {"postalCode", 40}, {"country", 120}} {
			value, err := rules.Text(source[field.name], field.name, field.max, false)
			if err != nil {
				return nil, err
			}
			address[field.name] = value
		}
		if (RecordValues{}).Truthy(source["type"]) {
			kind, err := rules.kind(source["type"])
			if err != nil {
				return nil, err
			}
			address["type"] = kind
		}
		addresses = append(addresses, address)
	}
	return addresses, nil
}

// unique answers the distinct non-empty texts of a list, at most maxItems.
func (rules ContactsRules) unique(input any, maxLength int, maxItems int) ([]any, error) {
	entries, _ := RecordValues{}.List(input)
	seen := map[string]bool{}
	unique := []any{}
	for _, entry := range entries {
		text, err := rules.Text(entry, "value", maxLength, false)
		if err != nil {
			return nil, err
		}
		if text != "" && !seen[text] {
			seen[text] = true
			unique = append(unique, text)
		}
	}
	if len(unique) > maxItems {
		unique = unique[:maxItems]
	}
	return unique, nil
}

// kind answers a value's type label: lower case, letters, digits and dashes.
func (rules ContactsRules) kind(value any) (string, error) {
	text, err := rules.Text(value, "type", 40, false)
	if err != nil {
		return "", err
	}
	return regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(strings.ToLower(text), ""), nil
}

// webAddress answers an http or https URL as the URL parser wrote it back.
func (ContactsRules) webAddress(value string) (string, error) {
	problem := errors.New("invalid contact URL")
	// The URL standard drops tabs and line breaks and escapes spaces.
	value = strings.ReplaceAll(regexp.MustCompile(`[\t\r\n]`).ReplaceAllString(value, ""), " ", "%20")
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", problem
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", problem
	}
	host := strings.ToLower(parsed.Hostname())
	if port := parsed.Port(); port != "" && !((scheme == "http" && port == "80") || (scheme == "https" && port == "443")) {
		host += ":" + port
	}
	if strings.Contains(parsed.Hostname(), ":") {
		host = "[" + strings.TrimPrefix(strings.TrimPrefix(host, "["), "]") + "]"
	}
	user := ""
	if parsed.User != nil {
		user = parsed.User.String() + "@"
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	href := scheme + "://" + user + host + path
	if parsed.RawQuery != "" || parsed.ForceQuery {
		href += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		href += "#" + parsed.EscapedFragment()
	}
	return href, nil
}

func (ContactsRules) mostValues() int { return 50 }
