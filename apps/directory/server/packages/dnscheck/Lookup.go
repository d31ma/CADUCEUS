package dnscheck

type Lookup interface {
	Mx(name string) ([]string, error)
	Txt(name string) ([]string, error)
}
