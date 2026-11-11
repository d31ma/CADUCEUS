package channels

import "mesail.dev/packages/outbox"

type outboxDocumentForTest struct{}

type outboxDocumentsForTest []outboxDocumentForTest

func (documents outboxDocumentsForTest) refs() []outbox.Document {
	return make([]outbox.Document, len(documents))
}
