import "strings"

// ContactsHelper keeps personal contacts and contact groups, always isolated
// by their exact mailbox owner. Writes act on the tenant storage of the
// record's domain.
// @Helper
type ContactsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// List answers the owner's contacts as {docId, ...contact}.
func (helper *ContactsHelper) List(ownerEmail string) ([]map[string]any, error) {
	return helper.owned(helper.contacts(), ownerEmail)
}

// owned answers the owner's contacts, read through store.
func (helper *ContactsHelper) owned(store FyloCollection, ownerEmail string) ([]map[string]any, error) {
	owner := strings.ToLower(ownerEmail)
	documents, err := store.Where(helper.ownerField(), owner)
	contacts := []map[string]any{}
	for _, document := range documents {
		contact := FyloRecord{Id: document.Id, Data: ContactsRecords{}.Hydrated(document.Data)}.WithDocId()
		if contact[helper.ownerField()] == owner {
			contacts = append(contacts, contact)
		}
	}
	return contacts, err
}

// FindById answers the document id and the owner's contact with the id, or
// "" and nil.
func (helper *ContactsHelper) FindById(id string, ownerEmail string) (string, map[string]any, error) {
	owner := strings.ToLower(ownerEmail)
	documents, err := helper.contacts().Where(helper.idField(), id)
	for _, document := range documents {
		contact := ContactsRecords{}.Hydrated(document.Data)
		if contact[helper.ownerField()] == owner {
			return document.Id, contact, nil
		}
	}
	return "", nil, err
}

// FindByEmail answers the owner's contact with an email address (compared
// without case), or nil.
func (helper *ContactsHelper) FindByEmail(ownerEmail string, address string) (map[string]any, error) {
	needle := strings.ToLower(address)
	contacts, err := helper.List(ownerEmail)
	for _, contact := range contacts {
		emails, _ := RecordValues{}.List(contact["emails"])
		for _, entry := range emails {
			value, _ := RecordValues{}.Object(entry)
			if text, isText := value["value"].(string); isText && strings.ToLower(text) == needle {
				return contact, nil
			}
		}
	}
	return nil, err
}

// Put stores a contact and answers its document id.
func (helper *ContactsHelper) Put(contact map[string]any) (string, error) {
	store, err := FyloTenancy{}.ForDomain(helper.contacts(), RecordValues{}.Text(contact[helper.domainField()]))
	if err != nil {
		return "", err
	}
	return store.Put(ContactsRecords{}.Serialized(contact))
}

// Replace writes a contact over its document.
func (helper *ContactsHelper) Replace(documentId string, contact map[string]any) error {
	store, err := FyloTenancy{}.ForDomain(helper.contacts(), RecordValues{}.Text(contact[helper.domainField()]))
	if err != nil {
		return err
	}
	return store.Patch(documentId, ContactsRecords{}.Serialized(contact))
}

// Delete deletes one contact document of a domain.
func (helper *ContactsHelper) Delete(documentId string, domain string) error {
	store, err := FyloTenancy{}.ForDomain(helper.contacts(), domain)
	if err != nil {
		return err
	}
	return store.Delete(documentId)
}

// ListGroups answers the owner's contact groups as {docId, ...group}.
func (helper *ContactsHelper) ListGroups(ownerEmail string) ([]map[string]any, error) {
	owner := strings.ToLower(ownerEmail)
	records, err := helper.groups().Where(helper.ownerField(), owner)
	owned := []map[string]any{}
	for _, group := range (StoredRecords{}).WithDocumentIds(records) {
		if group[helper.ownerField()] == owner {
			owned = append(owned, group)
		}
	}
	return owned, err
}

// FindGroupById answers the document id and the owner's group with the id,
// or "" and nil.
func (helper *ContactsHelper) FindGroupById(id string, ownerEmail string) (string, map[string]any, error) {
	groups, err := helper.ListGroups(strings.ToLower(ownerEmail))
	for _, group := range groups {
		if group[helper.idField()] == id {
			return RecordValues{}.Text(group[helper.documentField()]), StoredRecords{}.Without(group, helper.documentField()), nil
		}
	}
	return "", nil, err
}

// PutGroup stores a contact group and answers its document id.
func (helper *ContactsHelper) PutGroup(group map[string]any) (string, error) {
	store, err := FyloTenancy{}.ForDomain(helper.groups(), RecordValues{}.Text(group[helper.domainField()]))
	if err != nil {
		return "", err
	}
	return store.Put(group)
}

// ReplaceGroup writes a contact group over its document.
func (helper *ContactsHelper) ReplaceGroup(documentId string, group map[string]any) error {
	store, err := FyloTenancy{}.ForDomain(helper.groups(), RecordValues{}.Text(group[helper.domainField()]))
	if err != nil {
		return err
	}
	return store.Patch(documentId, group)
}

// DeleteGroup deletes a contact group and takes it off every contact of its
// owner.
func (helper *ContactsHelper) DeleteGroup(documentId string, group map[string]any) error {
	store, err := FyloTenancy{}.ForDomain(helper.groups(), RecordValues{}.Text(group[helper.domainField()]))
	if err != nil {
		return err
	}
	if err := store.Delete(documentId); err != nil {
		return err
	}
	domainContacts, err := FyloTenancy{}.ForDomain(helper.contacts(), RecordValues{}.Text(group[helper.domainField()]))
	if err != nil {
		return err
	}
	contacts, err := helper.owned(domainContacts, RecordValues{}.Text(group[helper.ownerField()]))
	if err != nil {
		return err
	}
	for _, contact := range contacts {
		if err := helper.leaveGroup(contact, group[helper.idField()]); err != nil {
			return err
		}
	}
	return nil
}

// leaveGroup takes one group off a contact that belongs to it.
func (helper *ContactsHelper) leaveGroup(contact map[string]any, groupId any) error {
	groupIds, _ := RecordValues{}.List(contact[helper.groupsField()])
	remaining := []any{}
	for _, id := range groupIds {
		if id != groupId {
			remaining = append(remaining, id)
		}
	}
	if len(remaining) == len(groupIds) {
		return nil
	}
	updated := StoredRecords{}.Without(contact, helper.documentField())
	updated[helper.groupsField()] = remaining
	updated["updatedAt"] = EpochTime{}.NowText()
	return helper.Replace(RecordValues{}.Text(contact[helper.documentField()]), updated)
}

func (ContactsHelper) idField() string { return "id" }

func (ContactsHelper) ownerField() string { return "ownerEmail" }

func (ContactsHelper) domainField() string { return "domain" }

func (ContactsHelper) groupsField() string { return "groupIds" }

func (ContactsHelper) documentField() string { return "docId" }

func (helper *ContactsHelper) contacts() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Contacts}
}

func (helper *ContactsHelper) groups() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().ContactGroups}
}
