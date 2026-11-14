import "cmp"

// @Service
type TenantCollectionsService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (service *TenantCollectionsService) Prepare(domainId string) error {
	membersFailure := service.documents.Ensure(service.names.Members(domainId))
	return cmp.Or(membersFailure, service.documents.Ensure(service.names.Addresses(domainId)))
}

func (service *TenantCollectionsService) Drop(domainId string, budget *WriteBudget) error {
	membersFailure := service.dropWithin(service.names.Members(domainId), budget)
	return cmp.Or(membersFailure, service.dropWithin(service.names.Addresses(domainId), budget))
}

func (service *TenantCollectionsService) dropWithin(collection string, budget *WriteBudget) error {
	exists, failure := service.documents.Exists(collection)
	return service.dropPresent(exists, failure, collection, budget)
}

func (service *TenantCollectionsService) dropPresent(exists bool, failure error, collection string, budget *WriteBudget) error {
	if failure != nil || !exists {
		return failure
	}
	return budget.Spend(service.oneWrite(), func() error { return service.documents.Drop(collection) })
}

func (service *TenantCollectionsService) oneWrite() int {
	return 1
}
