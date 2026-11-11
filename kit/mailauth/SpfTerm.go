package mailauth

import "net"

type spfTerm struct {
	name    string
	result  string
	domain  string
	network *net.IPNet
	prefix4 int
	prefix6 int
}

func (term spfTerm) outcome(matched bool) string {
	if matched {
		return term.result
	}
	return ""
}

func (term spfTerm) notDirective() bool {
	return term.name == "" || term.name == spfNames{}.redirect()
}

func (term spfTerm) notRedirect() bool {
	return term.name != spfNames{}.redirect()
}
