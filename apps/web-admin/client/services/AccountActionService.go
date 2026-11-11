import "strings"

// @Service
type AccountActionService struct {
	Dialog          string
	PersonId        string
	Name            string
	FirstName       string
	Username        string
	Busy            bool
	PasswordProblem string
	NameProblem     string
	ConfirmProblem  string
	DoneTitle       string
	DoneText        string
	Notice          string
	Problem         string
	// @Needs
	session *AdminSessionService
	// @Needs
	directoryClient *DirectoryClient
	// @Needs
	problems *AdminProblemService
	// @Needs
	rules                     *UsernameRuleService
	applicationEventPublisher ApplicationEventPublisher
}

func (account *AccountActionService) Ask(dialog string, personId string, name string, username string) {
	account.PersonId = personId
	account.Name = name
	account.FirstName = strings.Fields(name + " " + username)[0]
	account.Username = username
	account.PasswordProblem = ""
	account.NameProblem = ""
	account.ConfirmProblem = ""
	account.Problem = ""
	account.Notice = ""
	account.Dialog = dialog
}

func (account *AccountActionService) ResetWith(password string) {
	account.PasswordProblem = account.rules.PasswordProblem(password)
	account.callWhenValid(account.PasswordProblem, func() error {
		_, failure := account.directoryClient.ResetPassword(account.session.Authorization(), account.PersonId, PasswordReset{NewPassword: password})
		return failure
	})
	account.PasswordProblem = account.policyProblem(account.Problem, account.PasswordProblem)
	account.done("Password reset", account.FirstName+" is signed out everywhere. "+account.FirstName+" signs in with the new first password and then chooses a new one.")
}

func (account *AccountActionService) RemoveTwoStepCode() {
	account.call(func() error {
		_, failure := account.directoryClient.RemoveTwoStepCode(account.session.Authorization(), account.PersonId)
		return failure
	})
	account.done("2-step code removed", account.FirstName+" now signs in with the password only. "+account.FirstName+" can set up 2-step sign-in again in Settings.")
}

func (account *AccountActionService) TurnOff() {
	account.changeState("disabled")
	account.done("Account turned off", account.FirstName+" is signed out within a minute and cannot sign in to any domain. You can turn the account on again later.")
}

func (account *AccountActionService) TurnOn(personId string, name string) {
	account.Ask("", personId, name, "")
	account.changeState("active")
	account.Notice = account.noticeAfter(account.Problem, name+" can sign in again.")
	account.Dialog = account.dialogAfter(account.Problem, "")
}

func (account *AccountActionService) RenameWith(displayName string) {
	cleaned := strings.TrimSpace(displayName)
	account.NameProblem = account.emptyNameProblem(cleaned)
	account.callWhenValid(account.NameProblem, func() error {
		_, failure := account.directoryClient.ChangePerson(account.session.Authorization(), account.PersonId, PersonChange{DisplayName: cleaned})
		return failure
	})
	account.done("Name changed", "Everyone now sees "+cleaned+", in every domain of "+account.FirstName+".")
}

func (account *AccountActionService) DeleteWith(typed string) {
	account.ConfirmProblem = account.confirmProblem(strings.TrimSpace(typed))
	account.callWhenValid(account.ConfirmProblem, func() error {
		_, failure := account.directoryClient.DeletePerson(account.session.Authorization(), account.PersonId)
		return failure
	})
	account.done("Person deleted", account.FirstName+" can no longer sign in. Mesail now removes "+account.FirstName+" from every domain, in the background. The username "+account.Username+" is never given to anyone else.")
}

func (account *AccountActionService) Close() {
	account.Dialog = ""
}

func (account *AccountActionService) changeState(state string) {
	account.call(func() error {
		_, failure := account.directoryClient.ChangePerson(account.session.Authorization(), account.PersonId, PersonChange{State: state})
		return failure
	})
}

func (account *AccountActionService) callWhenValid(problem string, action func() error) {
	if problem != "" {
		account.Problem = "invalid"
		return
	}
	account.call(action)
}

func (account *AccountActionService) call(action func() error) {
	account.Busy = true
	account.Problem = account.problems.Of(action())
	account.Busy = false
	account.announce(account.Problem)
}

func (account *AccountActionService) done(title string, text string) {
	account.DoneTitle = title
	account.DoneText = text
	account.Dialog = account.dialogAfter(account.Problem, "done")
}

func (account *AccountActionService) dialogAfter(problem string, doneDialog string) string {
	dialog, known := map[string]string{"": doneDialog, "invalid": account.Dialog, "forbidden": "forbidden"}[problem]
	if !known {
		return "failed"
	}
	return dialog
}

func (account *AccountActionService) policyProblem(problem string, current string) string {
	if problem != "invalid" || current != "" {
		return current
	}
	return "Mesail did not accept this password. Use at least 12 characters, and not the username or the address."
}

func (account *AccountActionService) noticeAfter(problem string, notice string) string {
	if problem != "" {
		return ""
	}
	return notice
}

func (account *AccountActionService) emptyNameProblem(name string) string {
	if name != "" {
		return ""
	}
	return "Type the name people see, like Dan Example."
}

func (account *AccountActionService) confirmProblem(typed string) string {
	if typed == account.Username {
		return ""
	}
	return "Type " + account.Username + " to confirm."
}

func (account *AccountActionService) announce(problem string) {
	if problem != "" {
		return
	}
	account.applicationEventPublisher.PublishEvent(PeopleChanged{PersonId: account.PersonId}, false)
}
