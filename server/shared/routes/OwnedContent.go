import "net/http"

// OwnedContent holds the steps the signature and template routes share:
// finding one record the caller owns and renaming it without a clash.
type OwnedContent struct {
	Find    func(id string) (string, map[string]any, error)
	List    func(owner string) ([]FyloRecord, error)
	Update  func(documentId string, patch map[string]any) error
	Delete  func(documentId string) error
	Missing string
}

// Lookup answers the document id and record when the caller owns it, or
// the 404 answer.
func (content OwnedContent) Lookup(caller RouteCaller, id string) (string, map[string]any, *http.Response) {
	documentId, existing, err := content.Find(id)
	if err != nil {
		return "", nil, RouteAccess{}.Failure(err)
	}
	if documentId == "" || existing == nil || existing["owner"] != caller.Email {
		return "", nil, Answer{}.NotFound(content.Missing)
	}
	return documentId, existing, nil
}

// Touched makes a patch's name unique among the caller's other records and
// stamps it updated now.
func (content OwnedContent) Touched(caller RouteCaller, id string, patch map[string]any) *http.Response {
	if name, named := patch["name"].(string); named && name != "" {
		records, err := content.List(caller.Email)
		if err != nil {
			return RouteAccess{}.Failure(err)
		}
		patch["name"] = ContentNames{}.Unique(name, records, id)
	}
	patch["updatedAt"] = EpochTime{}.Now()
	return nil
}

// Saved stores a patch and answers the record as it now reads.
func (content OwnedContent) Saved(documentId string, existing map[string]any, patch map[string]any, present func(map[string]any) map[string]any) *http.Response {
	if err := content.Update(documentId, patch); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(present(ContentRecords{}.Merged(existing, patch)))
}

// Removed deletes the caller's record with an id and answers removed.
func (content OwnedContent) Removed(caller RouteCaller, id string, removed map[string]any) *http.Response {
	documentId, _, refusal := content.Lookup(caller, id)
	if refusal != nil {
		return refusal
	}
	if err := content.Delete(documentId); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(removed)
}

// Remove answers DELETE of one record by path: the caller must be allowed
// to change the mailbox and own the record.
func (content OwnedContent) Remove(access *AccessHelper, request *http.Request, id string, removed map[string]any) *http.Response {
	if _, refusal := (RequestBody{}).Value(request); refusal != nil {
		return refusal
	}
	caller, refusal := access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	if id == "" {
		return Answer{}.NotFound(content.Missing)
	}
	return content.Removed(caller, id, removed)
}

// Revised renames and stamps a patch, stores it and answers the record.
func (content OwnedContent) Revised(caller RouteCaller, documentId string, existing map[string]any, patch map[string]any, present func(map[string]any) map[string]any) *http.Response {
	if refusal := content.Touched(caller, RecordValues{}.Text(existing["id"]), patch); refusal != nil {
		return refusal
	}
	return content.Saved(documentId, existing, patch, present)
}
