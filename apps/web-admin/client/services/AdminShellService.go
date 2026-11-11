// @Service
type AdminShellService struct {
	View                      string
	DomainId                  string
	Pane                      string
	Announcement              string
	nextView                  string
	applicationEventPublisher ApplicationEventPublisher
	// @Needs
	session *AdminSessionService
}

// @OnStart
func (shell *AdminShellService) Start() {
	shell.View = ""
	shell.Pane = shell.listPane()
}

// @OnEvent
func (shell *AdminShellService) SignedIn(signedIn AdminSignedIn) {
	view := shell.firstView(signedIn, shell.nextView)
	shell.nextView = ""
	shell.Open(view, signedIn.DomainId)
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

func (shell *AdminShellService) OpenDomain(domainId string) {
	shell.nextView = "members"
	shell.session.SwitchTo(domainId)
}

func (shell *AdminShellService) OpenInstall() {
	shell.Open("domains", shell.session.DomainId)
}

func (shell *AdminShellService) ShowMenu() {
	shell.Pane = shell.listPane()
}

func (shell *AdminShellService) SidebarLabel(domainId string) string {
	return "Admin of " + domainId
}

func (shell *AdminShellService) TileLabel(domainId string, current bool) string {
	return domainId + map[bool]string{true: ", current domain", false: ""}[current]
}

func (shell *AdminShellService) Flag(value bool) string {
	return map[bool]string{true: "true", false: "false"}[value]
}

func (shell *AdminShellService) InstallView() bool {
	return map[string]bool{"domains": true, "people": true}[shell.View]
}

func (shell *AdminShellService) announced(message string) string {
	if message == "" {
		return shell.Announcement
	}
	return message
}

func (shell *AdminShellService) firstView(signedIn AdminSignedIn, nextView string) string {
	if nextView != "" {
		return nextView
	}
	return map[bool]string{true: "domains", false: "members"}[signedIn.Operator && !signedIn.DomainAdmin]
}

func (shell *AdminShellService) listPane() string {
	return "list"
}

func (shell *AdminShellService) mainPane() string {
	return "main"
}
