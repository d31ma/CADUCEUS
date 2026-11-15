type Duplicate struct {
	ErrorCase
}

func (Duplicate) UsernameTaken() *Duplicate { return Duplicate{}.taken(1030) }

func (Duplicate) DomainHosted(domainId string) *Duplicate {
	return Duplicate{}.takenWith(1012, domainId+" is already hosted")
}

func (Duplicate) AddressInUse(address string) *Duplicate {
	return Duplicate{}.takenWith(1018, address+" is already in use")
}

func (Duplicate) NameTaken(reason string, number int) *Duplicate {
	return Duplicate{}.takenWith(number, reason)
}

func (Duplicate) UsernameIsAddress(username string) *Duplicate {
	return Duplicate{}.takenWith(1025, username+" is used by an alias or a channel address")
}

func (Duplicate) UsernameRetired(username string) *Duplicate {
	return Duplicate{}.takenWith(1028, username+" belonged to a deleted person and is never reused")
}

func (Duplicate) AlreadyMember(username string, domainId string) *Duplicate {
	return Duplicate{}.takenWith(1029, username+" is already a member of "+domainId)
}

func (Duplicate) taken(number int) *Duplicate {
	return &Duplicate{ErrorCase{number, "duplicate", "", Duplicate{}.texts()[number]}}
}

func (Duplicate) takenWith(number int, text string) *Duplicate {
	return &Duplicate{ErrorCase{number, "duplicate", "", text}}
}

func (Duplicate) texts() map[int]string {
	return map[int]string{
		1030: "this username is already taken",
	}
}
