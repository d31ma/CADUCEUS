// TenantStorage runs work in one domain's tenant storage as the mail
// service, then acts as the caller again: the former
// FyloAccessContext.runForDomain.
type TenantStorage struct {
	Fylo *FyloHelper
}

// Within runs work acting for the domain.
func (storage TenantStorage) Within(domain string, work func() error) error {
	if storage.Fylo == nil {
		return work()
	}
	previous := storage.Fylo.ActForDomain(domain)
	defer storage.Fylo.Restore(previous)
	return work()
}
