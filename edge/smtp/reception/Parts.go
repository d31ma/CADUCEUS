package reception

type Parts struct {
	Checker AuthChecker
	Headers HeaderReader
	Store   RawMessageStore
	MailIn  MailInClient
}
