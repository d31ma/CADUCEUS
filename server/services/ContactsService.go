// ContactsService runs the contact lifecycle of a mailbox: its contacts,
// found-in-mail learning, groups, and vCard interchange.
// @Service
type ContactsService struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	contactsHelper *ContactsHelper
	// @Needs
	usersHelper *UsersHelper
}

// List answers the owner's contacts (and provider contacts) that match.
func (service *ContactsService) List(owner string, filters ContactFilters, provider []any) ([]map[string]any, error) {
	return service.book().List(owner, filters, provider)
}

// Get answers the owner's contact with an id, or nil.
func (service *ContactsService) Get(id string, owner string) (map[string]any, error) {
	return service.book().Get(id, owner)
}

// Create stores a new manual contact of the owner.
func (service *ContactsService) Create(owner string, input map[string]any) (map[string]any, error) {
	return service.book().Create(owner, input, "manual")
}

// Update merges input into the owner's contact, or answers nil.
func (service *ContactsService) Update(id string, owner string, input map[string]any) (map[string]any, error) {
	return service.book().Update(id, owner, input)
}

// Delete deletes the owner's contact and reports whether there was one.
func (service *ContactsService) Delete(id string, owner string) (bool, error) {
	return service.book().Delete(id, owner)
}

// Suggestions answers up to 20 completion addresses.
func (service *ContactsService) Suggestions(owner string, query string, provider []any) ([]map[string]any, error) {
	return service.book().Suggestions(owner, query, provider)
}

// ObserveParticipants learns found-in-mail contacts from the addresses of
// mail the owner sends or receives; failures only log.
func (service *ContactsService) ObserveParticipants(owner string, addresses []string, mailboxDomain string) {
	service.book().ObserveParticipants(owner, addresses, mailboxDomain)
}

// ImportVCard stores the contacts of a vCard file.
func (service *ContactsService) ImportVCard(owner string, content string, groupIds any) (map[string]any, error) {
	return service.book().Import(owner, content, groupIds)
}

// ExportVCard answers the owner's chosen contacts as a vCard file.
func (service *ContactsService) ExportVCard(owner string, ids []any) (string, int, error) {
	return service.book().Export(owner, ids)
}

// ListGroups answers the owner's groups sorted by name.
func (service *ContactsService) ListGroups(owner string) ([]map[string]any, error) {
	return service.book().Groups(owner)
}

// CreateGroup stores a new group.
func (service *ContactsService) CreateGroup(owner string, name any) (map[string]any, error) {
	return service.book().CreateGroup(owner, name)
}

// UpdateGroup renames a group, or answers nil.
func (service *ContactsService) UpdateGroup(id string, owner string, name any) (map[string]any, error) {
	return service.book().UpdateGroup(id, owner, name)
}

// DeleteGroup deletes a group and reports whether there was one.
func (service *ContactsService) DeleteGroup(id string, owner string) (bool, error) {
	return service.book().DeleteGroup(id, owner)
}

// Collection answers the contact collection routes.
func (service *ContactsService) Collection() ContactsRoute {
	return ContactsRoute{Access: service.accessHelper, Contacts: service}
}

// Item answers the routes of one contact.
func (service *ContactsService) Item() ContactRoute {
	return ContactRoute{Access: service.accessHelper, Contacts: service}
}

// Groups answers the contact group routes.
func (service *ContactsService) Groups() ContactGroupsRoute {
	return ContactGroupsRoute{Access: service.accessHelper, Contacts: service}
}

func (service *ContactsService) book() ContactBook {
	return ContactBook{Contacts: service.contactsHelper, Users: service.usersHelper}
}
