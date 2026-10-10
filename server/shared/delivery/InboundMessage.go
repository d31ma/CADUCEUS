// InboundMessage is one received message on its way into a mailbox: its id,
// domain, envelope, owner, subject, raw source, parsed body, thread
// metadata and spam verdict.
type InboundMessage struct {
	Id        string
	Domain    string
	Recipient string
	Sender    string
	Owner     string
	Subject   string
	Raw       string
	Parsed    ParsedMessage
	Metadata  map[string]any
	Spam      map[string]any
}
