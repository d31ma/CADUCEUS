// @Service
type AdminShellService struct {
	Heading string
	Summary string
}

// @OnStart
func (adminShell *AdminShellService) Start() {
	adminShell.Heading = "Mesail administration"
	adminShell.Summary = "Domains, channels and members"
}
