import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ContactBook is the contact lifecycle of a mailbox: its own contacts, the
// ones learned from mail it sends, its groups, and vCard import and export.
type ContactBook struct {
	Contacts *ContactsHelper
	Users    *UsersHelper
}

// List answers the owner's contacts (and provider contacts) that match the
// filters, sorted by name or first email.
func (book ContactBook) List(owner string, filters ContactFilters, provider []any) ([]map[string]any, error) {
	contacts, err := book.Contacts.List(owner)
	if err != nil {
		return nil, err
	}
	for _, contact := range provider {
		contacts = append(contacts, ContactsRules{}.ProviderContact(owner, contact))
	}
	needle := strings.ToLower(Security{}.Trim(filters.Query))
	matched := []map[string]any{}
	for _, contact := range contacts {
		groups, _ := RecordValues{}.List(contact["groupIds"])
		switch {
		case filters.Source != "" && contact["source"] != filters.Source:
		case filters.GroupId != "" && !book.includes(groups, filters.GroupId):
		case filters.Query != "" && !strings.Contains(ContactsRules{}.Searchable(contact), needle):
		default:
			matched = append(matched, StoredRecords{}.Without(contact, "docId"))
		}
	}
	sort.SliceStable(matched, func(left, right int) bool {
		return TextOrder{}.Compare(book.sortName(matched[left]), book.sortName(matched[right])) < 0
	})
	return matched, nil
}

// Get answers the owner's contact with an id, or nil.
func (book ContactBook) Get(id string, owner string) (map[string]any, error) {
	_, contact, err := book.Contacts.FindById(id, owner)
	return contact, err
}

// Create stores a new contact of the owner from a given source.
func (book ContactBook) Create(owner string, input map[string]any, source string) (map[string]any, error) {
	now := EpochTime{}.NowText()
	metadata := map[string]any{"id": "contact-" + Security{}.RandomUuid(), "source": source, "createdAt": now, "updatedAt": now}
	if source == "mail" {
		metadata["interactionCount"] = 1.0
		if count, isNumber := (RecordValues{}).Number(input["interactionCount"]); isNumber && count == float64(int64(count)) {
			metadata["interactionCount"] = count
		}
		metadata["lastContactedAt"] = RecordValues{}.Or(input["lastContactedAt"], now)
	}
	contact, err := ContactsRules{}.Normalized(owner, input, metadata)
	if err != nil {
		return nil, err
	}
	if err := book.assertGroups(owner, contact["groupIds"]); err != nil {
		return nil, err
	}
	if _, err := book.Contacts.Put(contact); err != nil {
		return nil, err
	}
	return contact, nil
}

// Update merges input into the owner's contact with an id and answers it,
// or nil when the owner has none.
func (book ContactBook) Update(id string, owner string, input map[string]any) (map[string]any, error) {
	documentId, current, err := book.Contacts.FindById(id, owner)
	if err != nil || documentId == "" || current == nil {
		return nil, err
	}
	merged := StoredRecords{}.Without(current)
	for key, value := range input {
		merged[key] = value
	}
	next, err := ContactsRules{}.Normalized(owner, merged, map[string]any{
		"id": current["id"], "source": current["source"], "createdAt": current["createdAt"], "updatedAt": EpochTime{}.NowText(),
		"interactionCount": current["interactionCount"], "lastContactedAt": current["lastContactedAt"],
	})
	if err != nil {
		return nil, err
	}
	if err := book.assertGroups(owner, next["groupIds"]); err != nil {
		return nil, err
	}
	if err := book.Contacts.Replace(documentId, next); err != nil {
		return nil, err
	}
	return next, nil
}

// Delete deletes the owner's contact with an id and reports whether there
// was one.
func (book ContactBook) Delete(id string, owner string) (bool, error) {
	documentId, contact, err := book.Contacts.FindById(id, owner)
	if err != nil || documentId == "" || contact == nil {
		return false, err
	}
	return true, book.Contacts.Delete(documentId, RecordValues{}.Text(contact["domain"]))
}

// Suggestions answers up to 20 addresses for completion: every email of the
// matching contacts, most contacted first, then most recently contacted.
func (book ContactBook) Suggestions(owner string, query string, provider []any) ([]map[string]any, error) {
	contacts, err := book.List(owner, ContactFilters{Query: query}, provider)
	if err != nil {
		return nil, err
	}
	values := RecordValues{}
	suggestions := []map[string]any{}
	for _, contact := range contacts {
		emails, _ := values.List(contact["emails"])
		for _, item := range emails {
			email, _ := values.Object(item)
			suggestions = append(suggestions, map[string]any{
				"contactId": contact["id"], "name": contact["name"], "email": email["value"], "source": contact["source"],
				"lastContactedAt": values.Or(contact["lastContactedAt"], ""), "interactionCount": values.Or(contact["interactionCount"], 0.0),
			})
		}
	}
	sort.SliceStable(suggestions, func(left, right int) bool {
		leftCount, rightCount := values.Numeric(suggestions[left]["interactionCount"]), values.Numeric(suggestions[right]["interactionCount"])
		if leftCount != rightCount {
			return rightCount < leftCount
		}
		return TextOrder{}.Compare(values.String(suggestions[right]["lastContactedAt"]), values.String(suggestions[left]["lastContactedAt"])) < 0
	})
	if len(suggestions) > 20 {
		suggestions = suggestions[:20]
	}
	return suggestions, nil
}

// ObserveParticipants learns found-in-mail contacts from the addresses of a
// message the owner sent: it counts one more interaction with each known
// contact and creates the others. A domain whose owner turned this off is
// skipped. Failures are logged and never stop mail delivery.
func (book ContactBook) ObserveParticipants(owner string, addresses []string, mailboxDomain string) {
	ownerDomain := mailboxDomain
	if ownerDomain == "" {
		ownerDomain, _ = ContactsRules{}.OwnerDomain(owner)
	}
	domain := Security{}.NormalizeDomain(ownerDomain)
	if book.Users != nil && domain != "" {
		_, user, _ := book.Users.FindByEmail(owner)
		preferences, _ := RecordValues{}.Object(user["mailPreferences"])
		disabled, _ := RecordValues{}.List(preferences["findInMailDisabledDomains"])
		if book.includes(disabled, domain) {
			return
		}
	}
	seen := map[string]bool{}
	for _, raw := range addresses {
		if seen[raw] {
			continue
		}
		seen[raw] = true
		email := Security{}.NormalizeEmailAddress(raw)
		if email == "" || email == strings.ToLower(owner) {
			continue
		}
		if err := book.observe(owner, email); err != nil {
			fmt.Fprintf(os.Stderr, "[contacts] found-in-mail update failed: %s\n", err)
		}
	}
}

func (book ContactBook) observe(owner string, email string) error {
	existing, err := book.Contacts.FindByEmail(owner, email)
	if err != nil {
		return err
	}
	now := EpochTime{}.NowText()
	if existing == nil {
		_, err := book.Create(owner, map[string]any{
			"name": "", "emails": []any{map[string]any{"value": email}}, "phones": []any{}, "addresses": []any{}, "urls": []any{}, "nicknames": []any{}, "groupIds": []any{},
			"interactionCount": 1.0, "lastContactedAt": now,
		}, "mail")
		return err
	}
	updated := StoredRecords{}.Without(existing, "docId")
	updated["interactionCount"] = RecordValues{}.NumberOf(RecordValues{}.Or(existing["interactionCount"], 0.0)) + 1
	updated["lastContactedAt"], updated["updatedAt"] = now, now
	return book.Contacts.Replace(RecordValues{}.Text(existing["docId"]), updated)
}

// Import stores every contact of a vCard file in the given groups and
// answers how many it imported and skipped.
func (book ContactBook) Import(owner string, content string, groupIds any) (map[string]any, error) {
	if err := book.assertGroups(owner, groupIds); err != nil {
		return nil, err
	}
	parsed, err := VCard{}.Parse(content)
	if err != nil {
		return nil, err
	}
	ids := []any{}
	skipped := 0
	for _, item := range parsed {
		input := StoredRecords{}.Without(item)
		input["groupIds"] = groupIds
		contact, err := book.Create(owner, input, "vcard")
		if err != nil {
			skipped++
			continue
		}
		ids = append(ids, contact["id"])
	}
	return map[string]any{"imported": len(ids), "skipped": skipped, "ids": ids}, nil
}

// Export answers the owner's contacts with the given ids (all of them when
// none are given) as one vCard file, and how many it holds.
func (book ContactBook) Export(owner string, ids []any) (string, int, error) {
	contacts, err := book.List(owner, ContactFilters{}, nil)
	if err != nil {
		return "", 0, err
	}
	chosen := contacts
	if len(ids) > 0 {
		chosen = []map[string]any{}
		for _, contact := range contacts {
			if book.includes(ids, contact["id"]) {
				chosen = append(chosen, contact)
			}
		}
	}
	return VCard{}.Serialized(chosen), len(chosen), nil
}

// Groups answers the owner's contact groups sorted by name.
func (book ContactBook) Groups(owner string) ([]map[string]any, error) {
	groups, err := book.Contacts.ListGroups(owner)
	if err != nil {
		return nil, err
	}
	listed := []map[string]any{}
	for _, group := range groups {
		listed = append(listed, StoredRecords{}.Without(group, "docId"))
	}
	sort.SliceStable(listed, func(left, right int) bool {
		return TextOrder{}.Compare(RecordValues{}.String(listed[left]["name"]), RecordValues{}.String(listed[right]["name"])) < 0
	})
	return listed, nil
}

// CreateGroup stores a new contact group with a name no other group of the
// owner has.
func (book ContactBook) CreateGroup(owner string, name any) (map[string]any, error) {
	normalized, err := ContactsRules{}.Text(name, "name", 120, true)
	if err != nil {
		return nil, err
	}
	groups, err := book.Groups(owner)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if strings.ToLower(RecordValues{}.String(group["name"])) == strings.ToLower(normalized) {
			return nil, errors.New("group name already exists")
		}
	}
	domain, err := ContactsRules{}.OwnerDomain(owner)
	if err != nil {
		return nil, err
	}
	now := EpochTime{}.NowText()
	group := map[string]any{"id": "group-" + Security{}.RandomUuid(), "ownerEmail": strings.ToLower(owner), "domain": domain, "name": normalized, "createdAt": now, "updatedAt": now}
	if _, err := book.Contacts.PutGroup(group); err != nil {
		return nil, err
	}
	return group, nil
}

// UpdateGroup renames the owner's group with an id and answers it, or nil
// when the owner has none.
func (book ContactBook) UpdateGroup(id string, owner string, name any) (map[string]any, error) {
	documentId, current, err := book.Contacts.FindGroupById(id, owner)
	if err != nil || documentId == "" || current == nil {
		return nil, err
	}
	normalized, err := ContactsRules{}.Text(name, "name", 120, true)
	if err != nil {
		return nil, err
	}
	next := StoredRecords{}.Without(current)
	next["name"], next["updatedAt"] = normalized, EpochTime{}.NowText()
	return next, book.Contacts.ReplaceGroup(documentId, next)
}

// DeleteGroup deletes the owner's group with an id (and takes it off every
// contact) and reports whether there was one.
func (book ContactBook) DeleteGroup(id string, owner string) (bool, error) {
	documentId, group, err := book.Contacts.FindGroupById(id, owner)
	found := err == nil && documentId != "" && group != nil
	if found {
		err = book.Contacts.DeleteGroup(documentId, group)
	}
	return found, err
}

// assertGroups refuses group ids the owner does not have.
func (book ContactBook) assertGroups(owner string, input any) error {
	ids, isList := RecordValues{}.List(input)
	if !isList {
		if text, isText := input.(string); isText && text != "" {
			return errors.New("groupIds.some is not a function")
		}
		return nil
	}
	if len(ids) == 0 {
		return nil
	}
	groups, err := book.Groups(owner)
	if err != nil {
		return err
	}
	owned := []any{}
	for _, group := range groups {
		owned = append(owned, group["id"])
	}
	for _, id := range ids {
		if !book.includes(owned, id) {
			return errors.New("contact group not found")
		}
	}
	return nil
}

func (ContactBook) sortName(contact map[string]any) string {
	values := RecordValues{}
	if name := values.String(values.Or(contact["name"], "")); name != "" {
		return name
	}
	emails, _ := values.List(contact["emails"])
	if len(emails) > 0 {
		email, _ := values.Object(emails[0])
		return values.String(values.Or(email["value"], ""))
	}
	return ""
}

func (ContactBook) includes(values []any, value any) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
