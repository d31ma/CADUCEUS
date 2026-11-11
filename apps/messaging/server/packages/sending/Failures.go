package sending

import "mesail.dev/packages/core"

type Failures struct{}

func (Failures) store(cause error) error {
	return &core.Unavailable{Message: "the message store is not available", Cause: cause}
}

func (Failures) directory(cause error) error {
	return &core.Unavailable{Message: "the directory is not available", Cause: cause}
}

func (Failures) attachments(cause error) error {
	return &core.Unavailable{Message: "the attachments service is not available", Cause: cause}
}

func (Failures) renderer(cause error) error {
	return &core.Unavailable{Message: "the text renderer is not available", Cause: cause}
}

func (Failures) invalid(message string, field string) error {
	return &core.InvalidRequest{Message: message, Field: field}
}

func (failures Failures) recipients(message string) error {
	return failures.invalid(message, "recipients")
}

func (failures Failures) missingSubject() error {
	return failures.invalid("an email needs a subject", "subject")
}

func (failures Failures) subjectRefusal(subject string) error {
	if subject != "" {
		return nil
	}
	return failures.missingSubject()
}

func (Failures) missingConversation() error {
	return &core.NotFound{Message: "no such conversation"}
}

func (Failures) missingMessage() error {
	return &core.NotFound{Message: "no such message"}
}

func (Failures) first(refusals []error) error {
	if len(refusals) == 0 {
		return nil
	}
	return refusals[0]
}
