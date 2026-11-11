package dnscheck

type Expected struct {
	MailHost     string
	SpfMechanism string
	DkimSelector string
}
