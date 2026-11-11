package recipients

type Destination struct {
	CallerDomainId  string
	AddressDomainId string
	Hosted          bool
	LocalKind       Kind
}
