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

func (service *TenantCollectionsService) Drop(domainId string) error {
	membersFailure := service.documents.Drop(service.names.Members(domainId))
	return cmp.Or(membersFailure, service.documents.Drop(service.names.Addresses(domainId)))
}
