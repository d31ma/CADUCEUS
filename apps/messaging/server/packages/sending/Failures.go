package sending

import "mesail.dev/packages/core"

type Failures struct{}

func (Failures) store(cause error) error {
	return core.Unavailable{}.Store(cause)
}

func (Failures) directory(cause error) error {
	return core.Unavailable{}.Directory(cause)
}

func (Failures) attachments(cause error) error {
	return core.Unavailable{}.AttachmentsService(cause)
}

func (Failures) renderer(cause error) error {
	return core.Unavailable{}.Renderer(cause)
}

func (Failures) subjectRefusal(subject string) error {
	if subject != "" {
		return nil
	}
	return core.InvalidRequest{}.MissingSubject()
}

func (Failures) missingConversation() error {
	return core.NotFound{}.NoConversation()
}

func (Failures) missingMessage() error {
	return core.NotFound{}.NoMessage()
}

func (Failures) first(refusals []error) error {
	if len(refusals) == 0 {
		return nil
	}
	return refusals[0]
}
