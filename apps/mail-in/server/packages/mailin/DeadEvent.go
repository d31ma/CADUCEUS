package mailin

type DeadEvent struct {
	Collection string
	DocumentId string
	EventId    string
	Type       string
	Subscriber string
	Attempts   int
	LastError  string
	DeadAt     string
}
