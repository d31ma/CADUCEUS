// @Service
type ShellService struct {
	ProductName string
	Heading     string
}

// @OnStart
func (shell *ShellService) Start() {
	shell.ProductName = "Mesail"
	shell.Heading = "Select a conversation"
}
