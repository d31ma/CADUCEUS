type Forbidden struct {
	ErrorCase
}

func (Forbidden) NotOperator() *Forbidden       { return Forbidden{}.refused(1002) }
func (Forbidden) NotMember() *Forbidden         { return Forbidden{}.refused(1004) }
func (Forbidden) DomainNotServed() *Forbidden   { return Forbidden{}.refused(1005) }
func (Forbidden) NotAccountManager() *Forbidden { return Forbidden{}.refused(1006) }

func (Forbidden) refused(number int) *Forbidden {
	return &Forbidden{ErrorCase{number, "forbidden", "", Forbidden{}.texts()[number]}}
}

func (Forbidden) texts() map[int]string {
	return map[int]string{
		1002: "only an Operator may do this",
		1004: "you are not a member of this domain",
		1005: "this domain is not hosted here",
		1006: "only an Operator or an admin of every domain of this person may change the account",
		1056: "a new device must be signed by one of your devices or by your backup key",
	}
}

func (Forbidden) DeviceNotSigned() *Forbidden {
	return Forbidden{}.refused(1056)
}
