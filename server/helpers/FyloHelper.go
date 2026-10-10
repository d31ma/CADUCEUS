// FyloHelper hands every helper of one request the same FYLO database. A
// route that authorized its caller makes the database act as that caller,
// so each operation carries the caller's POSIX access when the deployment
// enables it.
// @Helper
type FyloHelper struct {
	access *FyloAccess
}

// Database answers the request's database.
func (helper *FyloHelper) Database() FyloDatabase {
	settings := Settings{}
	return FyloDatabase{Boundary: Boundary{}.FromSettings(), Root: settings.Value("FYLO_ROOT"), Access: helper.access}
}

// ActFor makes later operations act as one mailbox of the given domains.
func (helper *FyloHelper) ActFor(email string, domains []string) {
	helper.access = FyloAccess{}.ForMailbox(email, domains)
}

// ActForDomain makes later operations act as the service of one domain's
// tenant storage (the former FyloAccessContext.runForDomain) and answers the
// access it replaced, for Restore.
func (helper *FyloHelper) ActForDomain(domain string) *FyloAccess {
	previous := helper.access
	helper.access = FyloAccess{}.ForDomain(domain)
	return previous
}

// Restore puts back the access ActForDomain replaced.
func (helper *FyloHelper) Restore(access *FyloAccess) {
	helper.access = access
}
