package mailout

type InvalidSwitch struct {
	Value string
}

func (invalid InvalidSwitch) Error() string {
	return "mail-out settings: mailout.directenabled, mailout.relayrequiretls and mailout.relayimplicittls must be true or false (as JSON or as the text \"true\" or \"false\"), got " + invalid.Value
}
