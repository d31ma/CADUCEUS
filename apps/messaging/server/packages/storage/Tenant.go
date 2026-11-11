package storage

import (
	"cmp"
	"encoding/json"
	"maps"

	"mesail.dev/packages/outbox"
)

type Tenant struct {
	Documents Documents
	Registry  Registry
	Drain     func(ref outbox.Ref)
}

func (tenant Tenant) Matching(collection string, domainId string, where map[string]any, target any) error {
	exists, failure := tenant.Documents.Exists(collection)
	return cmp.Or(failure, tenant.readIf(exists, collection, tenant.inDomain(where, domainId), target))
}

func (tenant Tenant) First(collection string, domainId string, where map[string]any, target any) (bool, error) {
	rows := []json.RawMessage{}
	failure := tenant.Matching(collection, domainId, where, &rows)
	return tenant.decodeFirst(rows, target), failure
}

func (tenant Tenant) Find(collection string, domainId string, id string, target any) (bool, error) {
	exists, failure := tenant.Documents.Exists(collection)
	content := json.RawMessage{}
	found, readFailure := tenant.findIf(exists, collection, id, &content)
	return tenant.decodeOwned(found, content, domainId, target), cmp.Or(failure, readFailure)
}

func (tenant Tenant) Insert(collection string, document any) (string, error) {
	failure := tenant.Documents.Ensure(collection)
	return tenant.insertUnless(failure, collection, document)
}

func (tenant Tenant) InsertTracked(collection string, document any, pending int) (string, error) {
	failure := cmp.Or(tenant.Documents.Ensure(collection), tenant.Registry.Register(collection))
	id, insertFailure := tenant.insertUnless(failure, collection, document)
	tenant.drainAfter(insertFailure, pending, outbox.Ref{Collection: collection, Id: id})
	return id, insertFailure
}

func (tenant Tenant) Replace(collection string, id string, document any) error {
	return tenant.Documents.Replace(collection, id, document)
}

func (tenant Tenant) ReplaceTracked(collection string, id string, document any, pending int) error {
	failure := tenant.Documents.Replace(collection, id, document)
	tenant.drainAfter(failure, pending, outbox.Ref{Collection: collection, Id: id})
	return failure
}

func (tenant Tenant) Delete(collection string, id string) error {
	return tenant.Documents.Delete(collection, id)
}

func (tenant Tenant) inDomain(where map[string]any, domainId string) map[string]any {
	copied := map[string]any{}
	maps.Copy(copied, where)
	copied[tenant.domainField()] = domainId
	return copied
}

func (tenant Tenant) readIf(exists bool, collection string, where map[string]any, target any) error {
	if !exists {
		return nil
	}
	return tenant.Documents.All(collection, where, target)
}

func (tenant Tenant) findIf(exists bool, collection string, id string, target any) (bool, error) {
	if !exists {
		return false, nil
	}
	return tenant.Documents.Find(collection, id, target)
}

func (Tenant) decodeFirst(rows []json.RawMessage, target any) bool {
	if len(rows) == 0 {
		return false
	}
	return json.Unmarshal(rows[0], target) == nil
}

func (Tenant) decodeOwned(found bool, content json.RawMessage, domainId string, target any) bool {
	owner := DomainOwner{}
	json.Unmarshal(content, &owner)
	if !found || owner.DomainId != domainId {
		return false
	}
	return json.Unmarshal(content, target) == nil
}

func (tenant Tenant) insertUnless(failure error, collection string, document any) (string, error) {
	if failure != nil {
		return "", failure
	}
	return tenant.Documents.Insert(collection, document)
}

func (tenant Tenant) drainAfter(failure error, pending int, ref outbox.Ref) {
	if failure != nil || pending == 0 || tenant.Drain == nil {
		return
	}
	tenant.Drain(ref)
}

func (Tenant) domainField() string {
	return "domainId"
}
