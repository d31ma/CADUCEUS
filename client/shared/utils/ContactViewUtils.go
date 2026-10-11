import (
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// ContactViewUtils computes what the contacts page shows: the list rows, the
// sidebar filters and groups, and the selected contact's details.
type ContactViewUtils struct{}

// Rows are the list rows, marking the selected and the checked contacts.
func (ContactViewUtils) Rows(contacts []Contact, selected string, checked []string) []ContactRow {
	return ContactViewUtils{}.rows(contacts, selected, checked)
}

func (ContactViewUtils) rows(contacts []Contact, selected string, checked []string) []ContactRow {
	rows := []ContactRow{}
	for _, contact := range contacts {
		rows = append(rows, ContactViewUtils{}.row(contact, contact.ID == selected, slices.Contains(checked, contact.ID)))
	}
	return rows
}

// heading is the name a contact goes by: its name, else its first email
// address, else its first phone number.
func (ContactViewUtils) heading(contact Contact) string {
	return TextUtils{}.FirstOf(contact.Name, ContactViewUtils{}.reachedAt(contact))
}

// PrimaryEmail is the contact's first email address, or "".
func (ContactViewUtils) PrimaryEmail(contact Contact) string {
	return ContactViewUtils{}.first(contact.Emails)
}

// initials are the two letters of the contact's avatar.
func (ContactViewUtils) initials(contact Contact) string {
	text := strings.TrimSpace(TextUtils{}.FirstOf(contact.Name, TextUtils{}.FirstOf(ContactViewUtils{}.PrimaryEmail(contact), ContactViewUtils{}.names().Question)))
	parts := strings.FieldsFunc(text, ContactViewUtils{}.separates)
	return strings.ToUpper(ContactViewUtils{}.initialsOf(text, parts))
}

// sourceLabel names where a contact came from.
func (ContactViewUtils) sourceLabel(source string) string {
	labels := map[string]string{"mail": "Found in mail", "vcard": "Imported vCard", "carddav": "Synced address book"}
	return TextUtils{}.FirstOf(labels[source], ContactViewUtils{}.names().Personal)
}

// addressLabel is one line of an address's filled-in parts.
func (ContactViewUtils) addressLabel(address ContactAddress) string {
	parts := []string{address.StreetAddress, address.ExtendedAddress, address.City, address.Region, address.PostalCode, address.Country}
	return strings.Join(slices.DeleteFunc(parts, ContactViewUtils{}.blank), ContactViewUtils{}.names().Comma)
}

// CountLabel is "1 contact" or "n contacts".
func (ContactViewUtils) CountLabel(count int) string {
	return strconv.Itoa(count) + ContactViewUtils{}.names().Space + map[bool]string{true: "contact", false: "contacts"}[count == 1]
}

// CheckedLabel is "n selected", or "" when nothing is checked.
func (ContactViewUtils) CheckedLabel(count int) string {
	return map[bool]string{false: strconv.Itoa(count) + " selected"}[count == 0]
}

// ExportLabel names what the export button exports.
func (ContactViewUtils) ExportLabel(count int) string {
	return map[bool]string{true: "Export all", false: "Export selected (" + strconv.Itoa(count) + ")"}[count == 0]
}

// Filters are the sidebar's source lists. A source list is active only while
// no group is chosen.
func (ContactViewUtils) Filters(source string, groupFilter string, countLabel string) []ContactFilter {
	active := map[bool]string{true: source, false: ContactViewUtils{}.names().Neither}[groupFilter == ""]
	return ContactViewUtils{}.marked(active, []ContactFilter{
		{ID: "", Label: "All contacts", Count: countLabel},
		{ID: "manual", Label: ContactViewUtils{}.names().Personal},
		{ID: "mail", Label: "Found in mail"},
		{ID: "vcard", Label: "Imported vCards"},
		{ID: "carddav", Label: "Synced address books"},
	})
}

func (ContactViewUtils) marked(active string, filters []ContactFilter) []ContactFilter {
	for index := range filters {
		filters[index].ClassName = ContactViewUtils{}.filterClass(filters[index].ID == active)
	}
	return filters
}

// GroupRows are the sidebar's groups with their rename and delete actions.
func (ContactViewUtils) GroupRows(groups []ContactGroup, groupFilter string) []ContactGroupRow {
	return ContactViewUtils{}.groupRows(groups, groupFilter)
}

func (ContactViewUtils) groupRows(groups []ContactGroup, groupFilter string) []ContactGroupRow {
	rows := []ContactGroupRow{}
	for _, group := range groups {
		rows = append(rows, ContactViewUtils{}.groupRow(group, group.ID == groupFilter))
	}
	return rows
}

// Detail is the selected contact for the detail panel.
func (ContactViewUtils) Detail(contact Contact, groups []ContactGroup) ContactDetail {
	email := ContactViewUtils{}.PrimaryEmail(contact)
	return ContactDetail{
		ID:             contact.ID,
		Heading:        ContactViewUtils{}.heading(contact),
		Initials:       ContactViewUtils{}.initials(contact),
		SourceLabel:    ContactViewUtils{}.sourceLabel(contact.Source),
		FullName:       strings.TrimSpace(contact.GivenName + ContactViewUtils{}.names().Space + contact.FamilyName),
		Company:        contact.Company,
		JobTitle:       contact.Title,
		Birthday:       contact.Birthday,
		Emails:         ContactViewUtils{}.links(contact.Emails, "email", "mailto:"),
		Phones:         ContactViewUtils{}.links(contact.Phones, "phone", "tel:"),
		Urls:           ContactViewUtils{}.links(contact.Urls, "website", ""),
		Addresses:      ContactViewUtils{}.addressLinks(contact.Addresses),
		GroupNames:     ContactViewUtils{}.groupNames(contact.Memberships, groups),
		Nicknames:      strings.Join(contact.Nicknames, ContactViewUtils{}.names().Comma),
		Notes:          contact.Notes,
		ShowProfile:    slices.ContainsFunc([]string{contact.GivenName, contact.FamilyName, contact.Company, contact.Title}, ContactViewUtils{}.filled),
		ShowNotes:      slices.Contains([]bool{len(contact.Nicknames) > 0, contact.Notes != ""}, true),
		ReadOnly:       contact.ReadOnly,
		CanMessage:     email != "",
		DeleteQuestion: ContactViewUtils{}.names().Delete + TextUtils{}.FirstOf(contact.Name, TextUtils{}.FirstOf(email, "this contact")) + ContactViewUtils{}.names().Question,
	}
}

func (ContactViewUtils) row(contact Contact, active bool, checked bool) ContactRow {
	return ContactRow{
		ID:          contact.ID,
		Heading:     ContactViewUtils{}.heading(contact),
		Subtitle:    ContactViewUtils{}.reachedAt(contact),
		Initials:    ContactViewUtils{}.initials(contact),
		SourceLabel: ContactViewUtils{}.sourceLabel(contact.Source),
		SourceClass: "contact-source source-" + contact.Source,
		ClassName:   map[bool]string{true: "contact-row active", false: "contact-row"}[active],
		Selected:    strconv.FormatBool(active),
		Checked:     checked,
		SelectLabel: "Select " + TextUtils{}.FirstOf(contact.Name, ContactViewUtils{}.PrimaryEmail(contact)),
	}
}

func (ContactViewUtils) groupRow(group ContactGroup, active bool) ContactGroupRow {
	return ContactGroupRow{
		ID:             group.ID,
		Name:           group.Name,
		ClassName:      ContactViewUtils{}.filterClass(active),
		RenameLabel:    "Rename " + group.Name,
		DeleteLabel:    ContactViewUtils{}.names().Delete + group.Name,
		DeleteQuestion: "Delete the “" + group.Name + "” group? Contacts will be kept.",
	}
}

func (ContactViewUtils) filterClass(active bool) string {
	return ContactViewUtils{}.names().Filter + map[bool]string{true: " active"}[active]
}

// reachedAt is the first email address, else the first phone number.
func (ContactViewUtils) reachedAt(contact Contact) string {
	return TextUtils{}.FirstOf(ContactViewUtils{}.first(contact.Emails), ContactViewUtils{}.first(contact.Phones))
}

func (ContactViewUtils) first(values []ContactValue) string {
	if len(values) == 0 {
		return ""
	}
	return values[0].Value
}

func (ContactViewUtils) separates(character rune) bool {
	return strings.ContainsRune("@._-", character) || unicode.IsSpace(character)
}

func (ContactViewUtils) initialsOf(text string, parts []string) string {
	if len(parts) > 1 {
		return string([]rune(parts[0])[:1]) + string([]rune(parts[1])[:1])
	}
	runes := []rune(text)
	return string(runes[:min(len(runes), 2)])
}

func (ContactViewUtils) blank(part string) bool {
	return part == ""
}

func (ContactViewUtils) filled(part string) bool {
	return part != ""
}

func (ContactViewUtils) links(values []ContactValue, fallback string, scheme string) []ContactLink {
	links := []ContactLink{}
	for _, value := range values {
		links = append(links, ContactLink{Type: TextUtils{}.FirstOf(value.Type, fallback), Value: value.Value, Href: scheme + value.Value})
	}
	return links
}

func (ContactViewUtils) addressLinks(addresses []ContactAddress) []ContactLink {
	links := []ContactLink{}
	for _, address := range addresses {
		links = append(links, ContactLink{Type: TextUtils{}.FirstOf(address.Type, "address"), Value: ContactViewUtils{}.addressLabel(address)})
	}
	return links
}

func (ContactViewUtils) groupNames(ids []string, groups []ContactGroup) []string {
	return ContactViewUtils{}.namesOf(ids, ContactViewUtils{}.groupIndex(groups))
}

func (ContactViewUtils) groupIndex(groups []ContactGroup) map[string]string {
	index := map[string]string{}
	for _, group := range groups {
		index[group.ID] = group.Name
	}
	return index
}

func (ContactViewUtils) namesOf(ids []string, index map[string]string) []string {
	names := []string{}
	for _, id := range ids {
		names = append(names, index[id])
	}
	return names
}

func (ContactViewUtils) names() ContactViewNames {
	return ContactViewNames{Space: " ", Comma: ", ", Question: "?", Delete: "Delete ", Filter: "contacts-filter", Neither: "\x00", Personal: "Personal"}
}
