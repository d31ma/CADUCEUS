// @Service
type ShellService struct {
	ProductName    string
	Heading        string
	Pane           string
	ThreadOpen     bool
	DomainMenuOpen bool
}

// @OnStart
func (shell *ShellService) Start() {
	shell.ProductName = "Mesail"
	shell.Heading = "Select a conversation"
	shell.Pane = shell.paneList()
}

func (shell *ShellService) ShowList() {
	shell.Pane = shell.paneList()
	shell.CloseMenus()
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
}

func (shell *ShellService) PaneFor(path string) string {
	return shell.pick(shell.onSettings(path), shell.paneMain(), shell.Pane)
}

func (shell *ShellService) HeadingFor(path string) string {
	return shell.pick(shell.onSettings(path), "Settings", shell.Heading)
}

func (shell *ShellService) OnSettings(path string) bool {
	return shell.onSettings(path)
}

func (shell *ShellService) onSettings(path string) bool {
	return path == "/settings"
}

func (shell *ShellService) pick(chosen bool, first string, otherwise string) string {
	if chosen {
		return first
	}
	return otherwise
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
