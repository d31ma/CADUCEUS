// @Service
type ShellService struct {
	ProductName    string
	Heading        string
	Pane           string
	Page           string
	ThreadOpen     bool
	DomainMenuOpen bool
	Ready          bool
	SessionChanges int
	// @Needs
	settings *SessionSettingsService
	// @Needs
	accounts *SessionAccountService
}

// @OnStart
func (shell *ShellService) Start() {
	shell.ProductName = "Mesail"
	shell.Heading = "Select a conversation"
	shell.Pane = shell.paneList()
}

// @OnEvent
func (shell *ShellService) Restored(restored SessionRestored) {
	shell.Ready = true
}

// @OnEvent
func (shell *ShellService) SessionChanged(changed SessionChanged) {
	shell.SessionChanges++
}

// @OnEvent
func (shell *ShellService) TwoStepLoaded(loaded SessionTwoStepLoaded) {
	shell.SessionChanges++
}

// @OnEvent
func (shell *ShellService) DomainChanged(changed SessionDomainChanged) {
	shell.CloseMenus()
}

func (shell *ShellService) ShowList() {
	shell.ShowPage("")
}

func (shell *ShellService) CloseThread() {
	shell.ThreadOpen = false
	shell.Pane = shell.paneMain()
}

func (shell *ShellService) ToggleDomainMenu() {
	shell.DomainMenuOpen = !shell.DomainMenuOpen
}

func (shell *ShellService) CloseMenus() {
	shell.DomainMenuOpen = false
	shell.accounts.MarkSeen()
}

func (shell *ShellService) ShowPage(page string) {
	shell.Page = page
	shell.Pane = shell.pick(page == "", shell.paneList(), shell.paneMain())
	shell.CloseMenus()
	shell.loadSettingsWhenShown(page)
}

func (shell *ShellService) loadSettingsWhenShown(page string) {
	if page != "settings" {
		return
	}
	shell.settings.RetryTwoStep()
}

func (shell *ShellService) Title() string {
	titles := map[string]string{"settings": "Settings", "activity": "Activity", "search": "Search"}
	return shell.pick(titles[shell.Page] == "", shell.Heading, titles[shell.Page])
}

func (shell *ShellService) pick(chosen bool, first string, otherwise string) string {
	if chosen {
		return first
	}
	return otherwise
}

func (shell *ShellService) FieldType(show bool) string {
	return map[bool]string{true: "text", false: "password"}[show]
}

func (shell *ShellService) Flag(value bool) string {
	return shell.flagText(value)
}

func (shell *ShellService) flagText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func (shell *ShellService) paneList() string {
	return "list"
}

func (shell *ShellService) paneMain() string {
	return "main"
}
