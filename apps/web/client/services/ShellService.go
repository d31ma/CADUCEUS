// @Service
type ShellService struct {
	ProductName     string
	Heading         string
	Pane            string
	ThreadOpen      bool
	DomainMenuOpen  bool
	AccountMenuOpen bool
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

func (shell *ShellService) ShowMain() {
	shell.Pane = shell.paneMain()
	shell.CloseMenus()
}

func (shell *ShellService) OpenThread() {
	shell.ThreadOpen = true
	shell.Pane = "thread"
}

func (shell *ShellService) CloseThread() {
	shell.ThreadOpen = false
	shell.Pane = shell.paneMain()
}

func (shell *ShellService) ToggleDomainMenu() {
	shell.DomainMenuOpen = !shell.DomainMenuOpen
	shell.AccountMenuOpen = false
}

func (shell *ShellService) ToggleAccountMenu() {
	shell.AccountMenuOpen = !shell.AccountMenuOpen
	shell.DomainMenuOpen = false
}

func (shell *ShellService) CloseMenus() {
	shell.DomainMenuOpen = false
	shell.AccountMenuOpen = false
}

func (shell *ShellService) paneList() string {
	return "list"
}

func (shell *ShellService) paneMain() string {
	return "main"
}
