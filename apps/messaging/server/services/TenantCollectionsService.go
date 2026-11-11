import (
	"cmp"
	"encoding/json"
	"maps"
)

// @Service
type TenantCollectionsService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	registry *OutboxCollectionsService
	// @Needs
	relay *OutboxRelayService
}

func (tenant *TenantCollectionsService) Matching(collection string, domainId string, where map[string]any, target any) error {
	exists, failure := tenant.documents.Exists(collection)
	return cmp.Or(failure, tenant.readIf(exists, collection, tenant.inDomain(where, domainId), target))
}

func (tenant *TenantCollectionsService) First(collection string, domainId string, where map[string]any, target any) (bool, error) {
	rows := []json.RawMessage{}
	failure := tenant.Matching(collection, domainId, where, &rows)
	return tenant.decodeFirst(rows, target), failure
}

func (tenant *TenantCollectionsService) Find(collection string, domainId string, id string, target any) (bool, error) {
	exists, failure := tenant.documents.Exists(collection)
	content := json.RawMessage{}
	found, readFailure := tenant.findIf(exists, collection, id, &content)
	return tenant.decodeOwned(found, content, domainId, target), cmp.Or(failure, readFailure)
}

func (tenant *TenantCollectionsService) Insert(collection string, document any) (string, error) {
	failure := tenant.documents.Ensure(collection)
	return tenant.insertUnless(failure, collection, document)
}

func (tenant *TenantCollectionsService) InsertTracked(collection string, document any, pending int) (string, error) {
	failure := cmp.Or(tenant.documents.Ensure(collection), tenant.registry.Register(collection))
	id, insertFailure := tenant.insertUnless(failure, collection, document)
	tenant.drainAfter(insertFailure, pending, collection, id)
	return id, insertFailure
}

func (tenant *TenantCollectionsService) Replace(collection string, id string, document any) error {
	return tenant.documents.Replace(collection, id, document)
}

func (tenant *TenantCollectionsService) ReplaceTracked(collection string, id string, document any, pending int) error {
	failure := tenant.documents.Replace(collection, id, document)
	tenant.drainAfter(failure, pending, collection, id)
	return failure
}

func (tenant *TenantCollectionsService) Delete(collection string, id string) error {
	return tenant.documents.Delete(collection, id)
}

func (tenant *TenantCollectionsService) inDomain(where map[string]any, domainId string) map[string]any {
	copied := map[string]any{}
	maps.Copy(copied, where)
	copied[tenant.domainField()] = domainId
	return copied
}

func (tenant *TenantCollectionsService) readIf(exists bool, collection string, where map[string]any, target any) error {
	if !exists {
		return nil
	}
	return tenant.documents.All(collection, where, target)
}

func (tenant *TenantCollectionsService) findIf(exists bool, collection string, id string, target any) (bool, error) {
	if !exists {
		return false, nil
	}
	return tenant.documents.Find(collection, id, target)
}

func (tenant *TenantCollectionsService) decodeFirst(rows []json.RawMessage, target any) bool {
	if len(rows) == 0 {
		return false
	}
	return json.Unmarshal(rows[0], target) == nil
}

func (tenant *TenantCollectionsService) decodeOwned(found bool, content json.RawMessage, domainId string, target any) bool {
	owner := DomainOwner{}
	json.Unmarshal(content, &owner)
	if !found || owner.DomainId != domainId {
		return false
	}
	return json.Unmarshal(content, target) == nil
}

func (tenant *TenantCollectionsService) insertUnless(failure error, collection string, document any) (string, error) {
	if failure != nil {
		return "", failure
	}
	return tenant.documents.Insert(collection, document)
}

func (tenant *TenantCollectionsService) drainAfter(failure error, pending int, collection string, id string) {
	if failure != nil || pending == 0 {
		return
	}
	tenant.relay.Drain(collection, id)
}

func (tenant *TenantCollectionsService) domainField() string {
	return "domainId"
}
