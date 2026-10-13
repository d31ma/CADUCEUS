// FyloTenancy makes a collection act as the service member of one managed
// domain's tenant storage (the former FyloAccessContext.runForDomain).
type FyloTenancy struct{}

// ForDomain answers the collection acting for the domain. With POSIX access
// on, a domain that is not valid is refused.
func (FyloTenancy) ForDomain(collection FyloCollection, domain string) (FyloCollection, error) {
	normalized := Security{}.NormalizeDomain(domain)
	access := FyloAccess{}.ForDomain(normalized)
	if access != nil && normalized == "" {
		return collection, &BoundaryError{Code: "EACCES", Message: "A valid domain is required for tenant storage"}
	}
	collection.Database = collection.Database.As(access)
	return collection, nil
}
