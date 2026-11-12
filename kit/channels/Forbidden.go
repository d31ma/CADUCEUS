package channels

type Forbidden struct {
	Message     string
	ErrorNumber int
}

func (Forbidden) NotADomainMember() *Forbidden {
	return &Forbidden{Message: "only members of a domain use its channels", ErrorNumber: 2002}
}

func (Forbidden) AddressNeedsAdmin() *Forbidden {
	return &Forbidden{Message: "only a domain admin can set a channel email address", ErrorNumber: 2012}
}

func (Forbidden) ArchiveNeedsAdminOrCreator() *Forbidden {
	return &Forbidden{Message: "only a domain admin or the channel creator can archive a channel", ErrorNumber: 2013}
}

func (Forbidden) TopicNeedsMember() *Forbidden {
	return &Forbidden{Message: "join the channel to change its topic", ErrorNumber: 2014}
}

func (failure *Forbidden) Error() string {
	return failure.Message
}

func (failure *Forbidden) Number() int {
	return failure.ErrorNumber
}
