package mailin

type RetryRequest struct {
	Collection string
	DocumentId string
	EventId    string
	Subscriber string
}
