// @Service
type DomainRemoveService struct {
	Confirming bool
	DomainId   string
	Name       string
	Typed      string
	CanRemove  bool
	Locked     bool
	Removing   bool
	Problem    string
	// @Needs
	session *AdminSessionService
	// @Needs
	directoryClient *DirectoryClient
	// @Needs
	problems *AdminProblemService
	// @Needs
	adminShell *AdminShellService
}

// @OnEvent({"condition":"value.view == 'domain'"})
func (remove *DomainRemoveService) Opened(opened AdminViewOpened) {
	remove.Cancel()
}

func (remove *DomainRemoveService) Start(domainId string, name string) {
	remove.Confirming = true
	remove.DomainId = domainId
	remove.Name = name
	remove.Type("")
}

func (remove *DomainRemoveService) Type(typed string) {
	remove.Typed = typed
	remove.CanRemove = typed == remove.Name
	remove.Locked = typed != remove.Name
	remove.Problem = ""
}

func (remove *DomainRemoveService) Cancel() {
	remove.Confirming = false
	remove.Type("")
}

func (remove *DomainRemoveService) Remove() {
	remove.Removing = true
	remove.Locked = true
	removed, failure := remove.removeWhenConfirmed(remove.CanRemove)
	remove.Removing = false
	remove.Locked = remove.Typed != remove.Name
	remove.Problem = remove.problemText(remove.problems.Of(failure))
	remove.closeWhenRemoved(removed)
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
