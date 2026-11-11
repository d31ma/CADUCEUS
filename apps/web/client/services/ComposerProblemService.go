// @Service
type ComposerProblemService struct{}

func (problems *ComposerProblemService) OfFailure(failure error) string {
	return problems.named(problems.statusOf(failure))
}

func (problems *ComposerProblemService) Text(problem string) string {
	return problems.texts()[problem]
}

func (problems *ComposerProblemService) statusOf(failure error) int {
	if failure == nil {
		return problems.noStatus()
	}
	return BehaviorErrorStatus(failure)
}

func (problems *ComposerProblemService) named(status int) string {
	known := map[int]string{
		problems.noStatus(): "",
		400:                 "invalidRequest",
		401:                 "signedOut",
		404:                 "notFound",
		409:                 "recipientsChanged",
		413:                 "tooLarge",
		422:                 "chatNotAllowed",
		429:                 "rateLimited",
	}
	problem, found := known[status]
	return problems.foundOrUnavailable(problem, found)
}

func (problems *ComposerProblemService) foundOrUnavailable(problem string, found bool) string {
	if found {
		return problem
	}
	return problems.unavailable()
}

func (problems *ComposerProblemService) texts() map[string]string {
	return map[string]string{
		"addRecipient":         "Add a recipient to send.",
		"tooManyRecipients":    "A message can go to at most 20 people. Remove some recipients, then send again.",
		"fixRecipients":        "Fix the recipients marked Not valid, then send again.",
		"waitForCheck":         "Wait until every address is checked, then send again.",
		"subjectMissing":       "Not sent yet. Add a subject, then send again.",
		"emptyMessage":         "Write a message or attach a file to send.",
		"waitForUploads":       "Wait until every file finishes uploading, then send again.",
		"fixAttachments":       "Remove the files that could not be uploaded, then send again.",
		"recipientsChanged":    "Not sent. Some recipients changed. Check how each one gets it, then send again.",
		"chatNotAllowed":       "Not sent. Chat stays inside your domain. Choose Email for people outside it.",
		"invalidRequest":       "Not sent. Check the recipients and the message, then send again.",
		"signedOut":            "Not sent. Your session ended. Sign in again, then send. Your message is kept here.",
		"notFound":             "Not sent. This conversation is not available any more.",
		"tooLarge":             "Not sent. The message or its files are too large.",
		"rateLimited":          "Not sent. You sent many messages in a short time. Wait a moment, then try again.",
		problems.unavailable(): "Not sent. Mesail could not be reached. Your message is kept here. Try again; it is sent only once.",
		"stillSending":         "Still sending. Wait a moment.",
		"resolveFailed":        "Some addresses could not be checked. Try again in a moment.",
	}
}

func (problems *ComposerProblemService) noStatus() int {
	return -1
}

func (problems *ComposerProblemService) unavailable() string {
	return "unavailable"
}
