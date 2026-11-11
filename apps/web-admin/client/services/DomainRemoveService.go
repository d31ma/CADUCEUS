import "strings"

// @Service
type DomainRemoveService struct {
	Confirming   bool
	DomainId     string
	Name         string
	ConfirmLabel string
	Typed        string
	Removing     bool
	NameProblem  string
	Problem      string
	// @Needs
	session *AdminSessionService
	// @Needs
	directoryClient *DirectoryClient
	// @Needs
	problems *AdminProblemService
	// @Needs
	adminShell *AdminShellService
}

// @OnEvent({"condition":"value.view == 'domain'","replay":true})
func (remove *DomainRemoveService) Opened(opened AdminViewOpened) {
	remove.Cancel()
}

func (remove *DomainRemoveService) Start(domainId string, name string) {
	remove.Confirming = true
	remove.DomainId = domainId
	remove.Name = name
	remove.ConfirmLabel = "Type " + name + " to confirm"
	remove.forget()
}

func (remove *DomainRemoveService) Cancel() {
	remove.Confirming = false
	remove.forget()
}

func (remove *DomainRemoveService) ConfirmWith(typed string) {
	remove.Typed = strings.TrimSpace(typed)
	remove.NameProblem = remove.nameProblem(remove.Typed)
	remove.Problem = ""
	remove.Removing = true
	removed, failure := remove.removeWhenConfirmed(remove.NameProblem == "")
	remove.Removing = false
	remove.Problem = remove.problemText(remove.problems.Of(failure))
	remove.closeWhenRemoved(removed)
}

func (remove *DomainRemoveService) forget() {
	remove.Typed = ""
	remove.NameProblem = ""
	remove.Problem = ""
}

func (remove *DomainRemoveService) nameProblem(typed string) string {
	if typed == remove.Name {
		return ""
	}
	return "The name does not match. Type " + remove.Name + " exactly to remove it."
}

func (remove *DomainRemoveService) removeWhenConfirmed(confirmed bool) (bool, error) {
	if !confirmed {
		return false, nil
	}
	_, failure := remove.directoryClient.RemoveDomain(remove.session.Authorization(), remove.DomainId)
	return failure == nil, failure
}

func (remove *DomainRemoveService) closeWhenRemoved(removed bool) {
	if !removed {
		return
	}
	remove.Cancel()
	remove.adminShell.Open("domains", remove.session.DomainId)
}

func (remove *DomainRemoveService) problemText(problem string) string {
	text, known := remove.problemTexts()[problem]
	if !known {
		return "The domain was not removed. Mesail could not reach the server. Try again."
	}
	return text
}

func (remove *DomainRemoveService) problemTexts() map[string]string {
	return map[string]string{
		"":          "",
		"forbidden": "Only operators can remove domains.",
		"notFound":  "This domain was already removed.",
	}
}
