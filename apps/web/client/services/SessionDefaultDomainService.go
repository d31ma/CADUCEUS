// @Service
type SessionDefaultDomainService struct {
	Problem       string
	ProblemDomain string
	Saved         string
	// @Needs
	directory *DirectoryClient
	// @Needs
	session *SessionService
	// @Needs
	accounts *SessionAccountService
	// @Needs
	problems *SessionProblemService
}

func (service *SessionDefaultDomainService) SaveWith(domainId string) {
	accessToken, failure := service.session.AccessToken()
	_, answerFailure := service.directory.SetDefaultDomain("Bearer "+accessToken, SessionDefaultDomain{DomainId: domainId})
	service.Problem = service.problems.OfDefaultDomain(service.firstFailure(failure, answerFailure))
	service.ProblemDomain = domainId
	service.Saved = ""
	service.keepWhenClear(domainId)
	service.removeWhenGone(domainId)
}

func (service *SessionDefaultDomainService) removeWhenGone(domainId string) {
	if service.Problem != "notMember" {
		return
	}
	service.accounts.Remove(domainId)
}

func (service *SessionDefaultDomainService) keepWhenClear(domainId string) {
	if service.Problem != "" {
		return
	}
	service.accounts.MarkDefault(domainId)
	service.Saved = domainId
}

func (service *SessionDefaultDomainService) firstFailure(failure error, answerFailure error) error {
	if failure != nil {
		return failure
	}
	return answerFailure
}
