import (
	"net/http"
	"net/url"
	"strings"
)

// ContactRoute answers /contacts/{id} and its mutation status: one contact
// of the caller, hosted or in a CardDAV address book.
type ContactRoute struct {
	Access   *AccessHelper
	Contacts *ContactsService
}

// Get answers the caller's contact, or their provider contact with the id.
func (route ContactRoute) Get(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	local, err := route.Contacts.Get(id, caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if local != nil {
		return Answer{}.Ok(local)
	}
	remote, err := route.provider().Find(caller.Token, id)
	if err != nil {
		return route.unexpected()
	}
	if remote == nil {
		return Answer{}.NotFound("Contact not found")
	}
	return Answer{}.Ok(remote)
}

// Update merges the body into the caller's contact, or queues the change on
// their provider contact.
func (route ContactRoute) Update(id string, request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if body == nil {
		body = map[string]any{}
	}
	local, err := route.Contacts.Update(id, caller.Email, body)
	if err != nil {
		return Answer{}.BadRequest(err.Error())
	}
	if local != nil {
		return Answer{}.Ok(local)
	}
	provider := route.provider()
	remote, err := provider.Find(caller.Token, id)
	if err != nil {
		return ContactsRoute{}.failure(err, "Invalid contact")
	}
	if remote == nil {
		return Answer{}.NotFound("Contact not found")
	}
	contact := StoredRecords{}.Without(body, "idempotencyKey")
	mutation, err := provider.Mutate(caller.Token, remote["accountId"], ProviderAccounts{}.Defined(map[string]any{
		"idempotencyKey": provider.IdempotencyKey(body["idempotencyKey"], "update"), "kind": "update", "contactId": remote["id"], "expectedEtag": remote["etag"], "contact": contact,
	}))
	if err != nil {
		return ContactsRoute{}.failure(err, "Invalid contact")
	}
	updated := StoredRecords{}.Without(remote)
	for key, value := range contact {
		updated[key] = value
	}
	updated["source"], updated["syncState"], updated["mutationId"] = "carddav", mutation["state"], mutation["id"]
	return Answer{}.Ok(ProviderAccounts{}.Defined(updated))
}

// Delete deletes the caller's contact, or queues the deletion of their
// provider contact.
func (route ContactRoute) Delete(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	deleted, err := route.Contacts.Delete(id, caller.Email)
	if err != nil {
		return route.unexpected()
	}
	if deleted {
		return Answer{}.Ok(map[string]any{"deleted": true})
	}
	provider := route.provider()
	remote, err := provider.Find(caller.Token, id)
	if err != nil {
		return ContactsRoute{}.failure(err, "Contact could not be deleted")
	}
	if remote == nil {
		return Answer{}.NotFound("Contact not found")
	}
	mutation, err := provider.Mutate(caller.Token, remote["accountId"], ProviderAccounts{}.Defined(map[string]any{
		"idempotencyKey": provider.IdempotencyKey(RouteQuery{}.Value(request, "idempotencyKey"), "delete"), "kind": "delete", "contactId": remote["id"], "expectedEtag": remote["etag"], "contact": map[string]any{},
	}))
	if err != nil {
		return ContactsRoute{}.failure(err, "Contact could not be deleted")
	}
	return Answer{}.Ok(ProviderAccounts{}.Defined(map[string]any{"deleted": false, "pending": true, "mutationId": mutation["id"], "accountId": remote["accountId"]}))
}

// Mutation answers the state of one queued change to a provider contact.
func (route ContactRoute) Mutation(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	accountId := RouteQuery{}.Value(request, "accountId")
	mutationId := route.lastSegment(request)
	if accountId == "" || id == "" || mutationId == "" {
		return Answer{}.BadRequest("accountId, contact id, and mutation id are required")
	}
	mutation, err := route.provider().Mutation(caller.Token, accountId, mutationId)
	if err != nil {
		if (ProviderAccounts{}).IsBoundary(err) {
			return ProviderAccounts{}.Failure(err)
		}
		return Answer{}.NotFound("Contact mutation not found")
	}
	if mutation["contactId"] != id {
		return Answer{}.NotFound("Contact mutation not found")
	}
	answered := StoredRecords{}.Without(mutation)
	answered["lastCode"] = RecordValues{}.Or(mutation["lastCode"], "")
	return Answer{}.Ok(answered)
}

// lastSegment answers the decoded last path segment: the mutation id of
// /contacts/{id}/mutations/{mutation_id}.
func (ContactRoute) lastSegment(request *http.Request) string {
	segments := strings.Split(strings.TrimSuffix(request.URL.EscapedPath(), "/"), "/")
	decoded, err := url.PathUnescape(segments[len(segments)-1])
	if err != nil {
		return ""
	}
	return decoded
}

// unexpected answers a failure the former route did not catch.
func (ContactRoute) unexpected() *http.Response {
	return Answer{}.Json(http.StatusInternalServerError, map[string]any{"error": "Internal server error"})
}

func (ContactRoute) provider() ProviderContacts {
	return ProviderContacts{Accounts: ProviderAccounts{}}
}
