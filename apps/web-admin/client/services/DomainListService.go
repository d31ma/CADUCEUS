import (
	"fmt"
	"slices"
)

// @Service
type DomainListService struct {
	Step        string
	Heading     string
	ProblemText string
	Rows        []DomainRow
	Removing    bool
	// @Needs
	session *AdminSessionService
	// @Needs
	directoryClient *DirectoryClient
	// @Needs
	problems *AdminProblemService
	// @Needs
	domainRows                *DomainRowService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnEvent({"condition":"value.view == 'domains'","replay":true})
// @InBackground
func (list *DomainListService) Opened(opened AdminViewOpened) {
	list.load()
	list.applicationEventPublisher.PublishEvent(AdminViewLoaded{View: opened.View, Message: list.Heading}, false)
}

func (list *DomainListService) Refresh() {
	list.load()
}

func (list *DomainListService) load() {
	list.Step = "loading"
	domains, failure := list.directoryClient.Domains(list.session.Authorization())
	problem := list.problems.Of(failure)
	list.Rows = list.rows(domains)
	list.Removing = slices.ContainsFunc(domains, list.removing)
	list.Heading = list.heading(len(domains))
	list.ProblemText = list.problemText(problem)
	list.Step = list.failedWhen(problem, list.emptyWhen(len(domains)))
}

func (list *DomainListService) rows(domains []Domain) []DomainRow {
	rows := []DomainRow{}
	for _, domain := range domains {
		rows = append(rows, list.domainRows.Row(domain))
	}
	return rows
}

func (list *DomainListService) removing(domain Domain) bool {
	return domain.State == "removing"
}

func (list *DomainListService) heading(count int) string {
	if count == 1 {
		return "1 domain"
	}
	return fmt.Sprintf("%d domains", count)
}

func (list *DomainListService) emptyWhen(count int) string {
	if count == 0 {
		return "empty"
	}
	return "ready"
}

func (list *DomainListService) failedWhen(problem string, fallback string) string {
	if problem == "" {
		return fallback
	}
	return "failed"
}

func (list *DomainListService) problemText(problem string) string {
	text, known := list.problemTexts()[problem]
	if !known {
		return "Mesail could not reach the server. Check your connection, then try again."
	}
	return text
}

func (list *DomainListService) problemTexts() map[string]string {
	return map[string]string{
		"":          "",
		"forbidden": "You need operator rights to manage domains.",
		"signedOut": "Your session ended. Sign in again to manage domains.",
	}
}
