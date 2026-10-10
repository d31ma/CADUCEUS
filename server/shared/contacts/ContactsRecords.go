import (
	"encoding/json"
	"fmt"
)

// ContactsRecords converts contact documents. FYLO cannot index arrays of
// objects, so each list of a contact (emails, phones ...) is stored as JSON
// text in a field named after it, such as emailsJson.
type ContactsRecords struct{}

// Serialized answers the document of a contact: without the repository's
// docId, and with every list as JSON text.
func (records ContactsRecords) Serialized(contact map[string]any) map[string]any {
	stored := StoredRecords{}.Without(contact, "docId")
	for _, field := range records.lists() {
		delete(stored, field)
		stored[field+records.suffix()] = StoredRecords{}.JsonText(RecordValues{}.Or(contact[field], []any{}))
	}
	return stored
}

// Hydrated answers one stored contact with its lists read back; a list that
// does not parse reads as empty.
func (records ContactsRecords) Hydrated(raw map[string]any) map[string]any {
	contact := StoredRecords{}.Without(raw)
	for _, field := range records.lists() {
		stored := field + records.suffix()
		var value any
		if json.Unmarshal([]byte(fmt.Sprint(RecordValues{}.Or(contact[stored], "[]"))), &value) != nil {
			value = []any{}
		}
		contact[field] = value
		delete(contact, stored)
	}
	return contact
}

func (ContactsRecords) lists() []string {
	return []string{"emails", "phones", "addresses", "urls", "nicknames", "groupIds"}
}

func (ContactsRecords) suffix() string { return "Json" }
