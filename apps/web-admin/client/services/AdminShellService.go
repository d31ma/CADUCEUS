// @Service
type AdminShellService struct {
	View                      string
	DomainId                  string
	Pane                      string
	applicationEventPublisher ApplicationEventPublisher
}

// @OnStart
func (shell *AdminShellService) Start() {
	shell.View = ""
	shell.Pane = shell.listPane()
}

// @OnEvent
func (shell *AdminShellService) SignedIn(signedIn AdminSignedIn) {
	shell.Open(shell.firstView(signedIn.Operator), signedIn.DomainId)
}

func (shell *AdminShellService) Open(view string, domainId string) {
	shell.View = view
	shell.DomainId = domainId
	shell.Pane = shell.mainPane()
	shell.applicationEventPublisher.PublishEvent(AdminViewOpened{View: view, DomainId: domainId}, false)
}

func (shell *AdminShellService) ShowMenu() {
	shell.Pane = shell.listPane()
}

func (shell *AdminShellService) Flag(value bool) string {
	return map[bool]string{true: "true", false: "false"}[value]
}

func (shell *AdminShellService) firstView(operator bool) string {
	if operator {
		return "domains"
	}
	return "members"
}

func (shell *AdminShellService) listPane() string {
	return "list"
}

func (shell *AdminShellService) mainPane() string {
	return "main"
}
