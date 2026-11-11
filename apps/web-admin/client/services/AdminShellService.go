// @Service
type AdminShellService struct {
	View                      string
	DomainId                  string
	Pane                      string
	Announcement              string
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

// @OnEvent
func (shell *AdminShellService) Loaded(loaded AdminViewLoaded) {
	shell.Announcement = shell.announced(loaded.Message)
}

func (shell *AdminShellService) Open(view string, domainId string) {
	shell.View = view
	shell.DomainId = domainId
	shell.Pane = shell.mainPane()
	shell.applicationEventPublisher.PublishEvent(AdminViewOpened{View: view, DomainId: domainId}, true)
}

func (shell *AdminShellService) ShowMenu() {
	shell.Pane = shell.listPane()
}

func (shell *AdminShellService) SidebarLabel(domainId string) string {
	return "Admin of " + domainId
}

func (shell *AdminShellService) Flag(value bool) string {
	return map[bool]string{true: "true", false: "false"}[value]
}

func (shell *AdminShellService) announced(message string) string {
	if message == "" {
		return shell.Announcement
	}
	return message
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
