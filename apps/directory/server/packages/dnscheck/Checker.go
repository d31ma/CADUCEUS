package dnscheck

type Checker struct {
	lookup Lookup
}

func NewChecker(lookup Lookup) Checker {
	return Checker{lookup: lookup}
}

func (checker Checker) Check(domain string, expected Expected) (States, error) {
	mailHosts, err := checker.lookup.Mx(domain)
	if err != nil {
		return States{}, err
	}
	domainTexts, err := checker.lookup.Txt(domain)
	if err != nil {
		return States{}, err
	}
	dkimTexts, err := checker.lookup.Txt(checker.dkimName(domain, expected.DkimSelector))
	if err != nil {
		return States{}, err
	}
	dmarcTexts, err := checker.lookup.Txt(checker.dmarcName(domain))
	if err != nil {
		return States{}, err
	}
	return States{
		Mx:    mxCheck{}.state(mailHosts, expected.MailHost),
		Spf:   spfCheck{}.state(domainTexts, expected.SpfMechanism),
		Dkim:  dkimCheck{}.state(dkimTexts),
		Dmarc: dmarcCheck{}.state(dmarcTexts),
	}, nil
}

func (checker Checker) dkimName(domain string, selector string) string {
	return selector + checker.dkimLabel() + domain
}

func (checker Checker) dmarcName(domain string) string {
	return checker.dmarcLabel() + domain
}

func (Checker) dkimLabel() string {
	return "._domainkey."
}

func (Checker) dmarcLabel() string {
	return "_dmarc."
}
