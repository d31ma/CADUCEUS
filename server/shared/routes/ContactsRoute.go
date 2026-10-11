import (
	"encoding/base64"
	"net/http"
)

// ContactsRoute answers the contact collection routes of the caller:
// /contacts, /contacts/suggestions, /contacts/import, /contacts/export and
// /contacts/address-books.
type ContactsRoute struct {
	Access   *AccessHelper
	Contacts *ContactsService
}

// List answers the caller's personal, found-in-mail and provider contacts
// that match the query.
func (route ContactsRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	source := RouteQuery{}.Value(request, "source")
	if source != "" && source != "manual" && source != "mail" && source != "vcard" && source != "carddav" {
		return Answer{}.BadRequest("source is invalid")
	}
	filters := ContactFilters{Query: ContactsRules{}.Prefix(RouteQuery{}.Value(request, "q"), 200), Source: source, GroupId: RouteQuery{}.Value(request, "groupId")}
	return route.answered(route.Contacts.List(caller.Email, filters, route.provider().Available(caller.Token)))
}

// Create stores one contact of the caller, or queues it on a CardDAV
// address book when the body names source=carddav.
func (route ContactsRoute) Create(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	values := RecordValues{}
	source := values.Coalesce(body["source"], "")
	accountId := values.Coalesce(body["accountId"], "")
	addressBookId := values.Coalesce(body["addressBookId"], "")
	contact := StoredRecords{}.Without(body, "source", "accountId", "addressBookId", "idempotencyKey")
	if source == "carddav" {
		provider := route.provider()
		if !provider.Identifier(accountId, "ext_") || !provider.Identifier(addressBookId, "book_") {
			return Answer{}.BadRequest("A valid CardDAV address book is required")
		}
		mutation, err := provider.Mutate(caller.Token, accountId, map[string]any{
			"idempotencyKey": provider.IdempotencyKey(body["idempotencyKey"], "create"), "kind": "create", "addressBookId": addressBookId, "contact": contact,
		})
		if err != nil {
			return route.failure(err, "Invalid contact")
		}
		created, _ := values.Object(mutation["desired"])
		created = StoredRecords{}.Without(created)
		for key, value := range map[string]any{
			"id": mutation["contactId"], "source": "carddav", "accountId": accountId, "addressBookId": addressBookId, "etag": "", "groupIds": []any{},
			"createdAt": mutation["createdAt"], "updatedAt": mutation["updatedAt"], "readOnly": false, "syncState": mutation["state"], "mutationId": mutation["id"],
		} {
			created[key] = value
		}
		return Answer{}.Ok(StoredRecords{}.Defined(created))
	}
	if values.Truthy(source) {
		return Answer{}.BadRequest("source is invalid")
	}
	created, err := route.Contacts.Create(caller.Email, contact)
	if err != nil {
		return Answer{}.BadRequest(err.Error())
	}
	return Answer{}.Ok(created)
}

// Suggestions answers completion addresses for the query.
func (route ContactsRoute) Suggestions(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	query := ContactsRules{}.Prefix(RouteQuery{}.Value(request, "q"), 200)
	return route.answered(route.Contacts.Suggestions(caller.Email, query, route.provider().Available(caller.Token)))
}

// Import stores the contacts of a vCard file.
func (route ContactsRoute) Import(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	content, isText := body["content"].(string)
	if !isText || (Security{}).Trim(content) == "" {
		return Answer{}.BadRequest("vCard content is required")
	}
	imported, err := route.Contacts.ImportVCard(caller.Email, content, RecordValues{}.Or(body["groupIds"], []any{}))
	if err != nil {
		return Answer{}.BadRequest(err.Error())
	}
	return Answer{}.Ok(imported)
}

// Export answers the caller's chosen contacts as a base64 vCard file.
func (route ContactsRoute) Export(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	ids := []any{}
	if value, present := body["ids"]; present {
		list, isList := RecordValues{}.List(value)
		if !isList {
			return Answer{}.BadRequest("ids must be an array")
		}
		ids = list
	}
	content, count, err := route.Contacts.ExportVCard(caller.Email, ids)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	filename := "contacts.vcf"
	if count == 1 {
		filename = "contact.vcf"
	}
	return Answer{}.Ok(map[string]any{
		"filename": filename, "contentType": "text/vcard; charset=utf-8", "contentBase64": base64.StdEncoding.EncodeToString([]byte(content)), "count": count,
	})
}

// AddressBooks answers the caller's CardDAV address books.
func (route ContactsRoute) AddressBooks(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	books, err := route.provider().Books(caller.Token)
	if err != nil {
		return ExternalAccounts{}.Failure(err)
	}
	return Answer{}.Ok(books)
}

// answered answers a stored value, or the storage failure.
func (ContactsRoute) answered(value any, err error) *http.Response {
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(value)
}

// failure answers a failed contact change: boundary failures with their
// status, anything else as a bad request.
func (ContactsRoute) failure(err error, fallback string) *http.Response {
	if (ExternalAccounts{}).IsBoundary(err) {
		return ExternalAccounts{}.Failure(err)
	}
	if err.Error() == "" {
		return Answer{}.BadRequest(fallback)
	}
	return Answer{}.BadRequest(err.Error())
}

func (ContactsRoute) provider() ProviderContacts {
	return ProviderContacts{Accounts: ExternalAccounts{}.FromSettings()}
}
