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

func (shell *ShellService) paneList() string {
	return "list"
}

func (shell *ShellService) paneMain() string {
	return "main"
}
