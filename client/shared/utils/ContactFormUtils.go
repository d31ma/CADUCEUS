import (
	"slices"
	"strconv"
	"strings"
)

// ContactFormUtils turns a contact into the editor's form, edits the form's
// rows, and turns the form into the request body, the way the contacts page
// did. Rows carry keys made from seed, so the view can tell them apart.
type ContactFormUtils struct{}

// Blank is the form of a new contact: one empty row of each kind.
func (ContactFormUtils) Blank(seed string) ContactForm {
	return ContactForm{
		Emails:      ContactFormUtils{}.editableValues(nil, ContactFormUtils{}.seeded(seed, ContactFormUtils{}.names().Emails)),
		Phones:      ContactFormUtils{}.editableValues(nil, ContactFormUtils{}.seeded(seed, ContactFormUtils{}.names().Phones)),
		Urls:        ContactFormUtils{}.editableValues(nil, ContactFormUtils{}.seeded(seed, ContactFormUtils{}.names().Urls)),
		Addresses:   ContactFormUtils{}.editableAddresses(nil, ContactFormUtils{}.seeded(seed, ContactFormUtils{}.names().Addresses)),
		Memberships: []string{},
	}
}

// FromContact is the form that edits contact.
func (ContactFormUtils) FromContact(contact Contact, seed string) ContactForm {
	return ContactForm{
		Name:        contact.Name,
		GivenName:   contact.GivenName,
		FamilyName:  contact.FamilyName,
		Company:     contact.Company,
		Title:       contact.Title,
		Birthday:    contact.Birthday,
		Notes:       contact.Notes,
		Nicknames:   strings.Join(contact.Nicknames, ", "),
		Emails:      ContactFormUtils{}.editableValues(contact.Emails, ContactFormUtils{}.seeded(seed, ContactFormUtils{}.names().Emails)),
		Phones:      ContactFormUtils{}.editableValues(contact.Phones, ContactFormUtils{}.seeded(seed, ContactFormUtils{}.names().Phones)),
		Urls:        ContactFormUtils{}.editableValues(contact.Urls, ContactFormUtils{}.seeded(seed, ContactFormUtils{}.names().Urls)),
		Addresses:   ContactFormUtils{}.editableAddresses(contact.Addresses, ContactFormUtils{}.seeded(seed, ContactFormUtils{}.names().Addresses)),
		Memberships: append([]string{}, contact.Memberships...),
	}
}

// Draft is the request body of the form: trimmed text, and only the rows and
// address parts that were filled in.
func (ContactFormUtils) Draft(form ContactForm) ContactDraft {
	return ContactDraft{
		Name:        strings.TrimSpace(form.Name),
		GivenName:   strings.TrimSpace(form.GivenName),
		FamilyName:  strings.TrimSpace(form.FamilyName),
		Company:     strings.TrimSpace(form.Company),
		Title:       strings.TrimSpace(form.Title),
		Birthday:    strings.TrimSpace(form.Birthday),
		Notes:       strings.TrimSpace(form.Notes),
		Nicknames:   ContactFormUtils{}.nicknames(form.Nicknames),
		Emails:      ContactFormUtils{}.draftValues(form.Emails),
		Phones:      ContactFormUtils{}.draftValues(form.Phones),
		Urls:        ContactFormUtils{}.draftValues(form.Urls),
		Addresses:   ContactFormUtils{}.draftAddresses(form.Addresses),
		Memberships: append([]string{}, form.Memberships...),
	}
}

// Problem is why the draft cannot be saved, or "" when it can.
func (ContactFormUtils) Problem(draft ContactDraft) string {
	reachable := slices.Contains([]bool{draft.Name != "", len(draft.Emails) > 0, len(draft.Phones) > 0}, true)
	return map[bool]string{false: "Add a name, email address, or phone number."}[reachable]
}

// WithField sets one of the form's text fields, such as "givenName".
func (ContactFormUtils) WithField(form ContactForm, field string, value string) ContactForm {
	ContactFormUtils{}.assign(ContactFormUtils{}.textFields(&form)[field], value)
	return form
}

// WithValue sets the type or the value of one email, phone or website row.
func (ContactFormUtils) WithValue(form ContactForm, edit ValueEdit) ContactForm {
	list := ContactFormUtils{}.listOf(&form, edit.Kind)
	*list = slices.Clone(*list)
	row := ContactFormUtils{}.valueAt(*list, edit.Key)
	ContactFormUtils{}.assign(map[string]*string{ContactFormUtils{}.names().Type: &row.Type, "value": &row.Value}[edit.Field], edit.Value)
	return form
}

// WithAddedValue adds an empty row of one kind.
func (ContactFormUtils) WithAddedValue(form ContactForm, kind string, seed string) ContactForm {
	list := ContactFormUtils{}.listOf(&form, kind)
	*list = append(slices.Clone(*list), ContactFormUtils{}.blankValue(seed))
	return form
}

// WithoutValue removes one row; the last row of a kind empties instead.
func (ContactFormUtils) WithoutValue(form ContactForm, kind string, key string, seed string) ContactForm {
	list := ContactFormUtils{}.listOf(&form, kind)
	kept := slices.DeleteFunc(slices.Clone(*list), func(row EditableValue) bool { return row.Key == key })
	*list = append(kept, ContactFormUtils{}.blankValue(seed))[:max(len(kept), 1)]
	return form
}

// WithAddress sets one part of one address, such as "city".
func (ContactFormUtils) WithAddress(form ContactForm, key string, field string, value string) ContactForm {
	form.Addresses = slices.Clone(form.Addresses)
	address := ContactFormUtils{}.addressAt(form.Addresses, key)
	ContactFormUtils{}.assign(ContactFormUtils{}.addressFields(address)[field], value)
	return form
}

// WithAddedAddress adds an empty address.
func (ContactFormUtils) WithAddedAddress(form ContactForm, seed string) ContactForm {
	form.Addresses = append(slices.Clone(form.Addresses), ContactFormUtils{}.blankAddress(seed))
	return form
}

// WithoutAddress removes one address; the last address empties instead.
func (ContactFormUtils) WithoutAddress(form ContactForm, key string, seed string) ContactForm {
	kept := slices.DeleteFunc(slices.Clone(form.Addresses), func(address EditableAddress) bool { return address.Key == key })
	form.Addresses = append(kept, ContactFormUtils{}.blankAddress(seed))[:max(len(kept), 1)]
	return form
}

// Toggled adds id to ids, once, or removes it.
func (ContactFormUtils) Toggled(ids []string, id string, included bool) []string {
	return ContactFormUtils{}.toggled(ids, id, included)
}

func (ContactFormUtils) toggled(ids []string, id string, included bool) []string {
	kept := slices.DeleteFunc(slices.Clone(ids), func(candidate string) bool { return candidate == id })
	if included {
		return append(kept, id)
	}
	return kept
}

func (ContactFormUtils) blankValue(seed string) EditableValue {
	return EditableValue{Key: IdentityUtils{}.UUID(seed)}
}

func (ContactFormUtils) blankAddress(seed string) EditableAddress {
	return EditableAddress{Key: IdentityUtils{}.UUID(seed)}
}

// editableValues keys the rows of one kind; a kind with no rows gets an
// empty one.
func (ContactFormUtils) editableValues(values []ContactValue, seed string) []EditableValue {
	rows := []EditableValue{}
	for index, value := range append(slices.Clip(values), (ContactValue{}))[:max(len(values), 1)] {
		rows = append(rows, EditableValue{Key: ContactFormUtils{}.key(seed, index), Type: value.Type, Value: value.Value})
	}
	return rows
}

// editableAddresses keys the addresses; a contact with none gets an empty one.
func (ContactFormUtils) editableAddresses(addresses []ContactAddress, seed string) []EditableAddress {
	keyed := []EditableAddress{}
	for index, address := range (ContactFormUtils{}).padded(addresses) {
		keyed = append(keyed, ContactFormUtils{}.editableAddress(address, ContactFormUtils{}.key(seed, index)))
	}
	return keyed
}

func (ContactFormUtils) padded(addresses []ContactAddress) []ContactAddress {
	return append(slices.Clip(addresses), ContactAddress{})[:max(len(addresses), 1)]
}

func (ContactFormUtils) editableAddress(address ContactAddress, key string) EditableAddress {
	return EditableAddress{
		Key:             key,
		Type:            address.Type,
		StreetAddress:   address.StreetAddress,
		ExtendedAddress: address.ExtendedAddress,
		City:            address.City,
		Region:          address.Region,
		PostalCode:      address.PostalCode,
		Country:         address.Country,
	}
}

func (ContactFormUtils) key(seed string, index int) string {
	return IdentityUtils{}.UUID(ContactFormUtils{}.seeded(seed, strconv.Itoa(index)))
}

// listOf is the form's row list of one kind; an unknown kind edits a list
// that is thrown away.
func (ContactFormUtils) listOf(form *ContactForm, kind string) *[]EditableValue {
	names := ContactFormUtils{}.names()
	list, known := map[string]*[]EditableValue{names.Emails: &form.Emails, names.Phones: &form.Phones, names.Urls: &form.Urls}[kind]
	if !known {
		return &[]EditableValue{}
	}
	return list
}

// valueAt is the row with key; an unknown key edits a row that is thrown away.
func (ContactFormUtils) valueAt(rows []EditableValue, key string) *EditableValue {
	index := slices.IndexFunc(rows, func(row EditableValue) bool { return row.Key == key })
	if index < 0 {
		return &EditableValue{}
	}
	return &rows[index]
}

func (ContactFormUtils) addressAt(addresses []EditableAddress, key string) *EditableAddress {
	index := slices.IndexFunc(addresses, func(address EditableAddress) bool { return address.Key == key })
	if index < 0 {
		return &EditableAddress{}
	}
	return &addresses[index]
}

func (ContactFormUtils) textFields(form *ContactForm) map[string]*string {
	return map[string]*string{
		"name":       &form.Name,
		"givenName":  &form.GivenName,
		"familyName": &form.FamilyName,
		"company":    &form.Company,
		"title":      &form.Title,
		"birthday":   &form.Birthday,
		"notes":      &form.Notes,
		"nicknames":  &form.Nicknames,
	}
}

func (ContactFormUtils) addressFields(address *EditableAddress) map[string]*string {
	return map[string]*string{
		ContactFormUtils{}.names().Type: &address.Type,
		"streetAddress":                 &address.StreetAddress,
		"extendedAddress":               &address.ExtendedAddress,
		"city":                          &address.City,
		"region":                        &address.Region,
		"postalCode":                    &address.PostalCode,
		"country":                       &address.Country,
	}
}

// assign sets a field that the view named; an unknown name changes nothing.
func (ContactFormUtils) assign(field *string, value string) {
	if field != nil {
		*field = value
	}
}

func (ContactFormUtils) nicknames(text string) []string {
	names := []string{}
	for _, name := range strings.Split(text, ",") {
		names = append(names, strings.TrimSpace(name))
	}
	return slices.DeleteFunc(names, func(name string) bool { return name == "" })
}

func (ContactFormUtils) draftValues(rows []EditableValue) []DraftValue {
	values := []DraftValue{}
	for _, row := range rows {
		values = append(values, DraftValue{Value: strings.TrimSpace(row.Value), Type: strings.TrimSpace(row.Type)})
	}
	return slices.DeleteFunc(values, func(value DraftValue) bool { return value.Value == "" })
}

// draftAddresses keeps the addresses that have more than a type.
func (ContactFormUtils) draftAddresses(rows []EditableAddress) []DraftAddress {
	addresses := []DraftAddress{}
	for _, row := range rows {
		addresses = append(addresses, DraftAddress{
			Type:            strings.TrimSpace(row.Type),
			StreetAddress:   strings.TrimSpace(row.StreetAddress),
			ExtendedAddress: strings.TrimSpace(row.ExtendedAddress),
			City:            strings.TrimSpace(row.City),
			Region:          strings.TrimSpace(row.Region),
			PostalCode:      strings.TrimSpace(row.PostalCode),
			Country:         strings.TrimSpace(row.Country),
		})
	}
	return slices.DeleteFunc(addresses, ContactFormUtils{}.bare)
}

func (ContactFormUtils) bare(address DraftAddress) bool {
	return address.StreetAddress+address.ExtendedAddress+address.City+address.Region+address.PostalCode+address.Country == ""
}

func (ContactFormUtils) seeded(seed string, part string) string {
	return seed + ContactFormUtils{}.names().Separator + part
}

func (ContactFormUtils) names() ContactFormNames {
	return ContactFormNames{Emails: "emails", Phones: "phones", Urls: "urls", Addresses: "addresses", Type: "type", Separator: "|"}
}
