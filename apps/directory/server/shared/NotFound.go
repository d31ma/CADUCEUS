type NotFound struct {
	ErrorCase
}

func (NotFound) Hidden() *NotFound           { return NotFound{}.missing(1003) }
func (NotFound) Address() *NotFound          { return NotFound{}.missing(1015) }
func (NotFound) Member() *NotFound           { return NotFound{}.missing(1017) }
func (NotFound) ChannelAddress() *NotFound   { return NotFound{}.missing(1021) }
func (NotFound) Account() *NotFound          { return NotFound{}.missing(1032) }
func (NotFound) Person() *NotFound           { return NotFound{}.missing(1038) }
func (NotFound) PersonDeleted() *NotFound    { return NotFound{}.missing(1039) }
func (NotFound) SeedTurnedOff() *NotFound    { return NotFound{}.missing(1040) }
func (NotFound) DeadEvent() *NotFound        { return NotFound{}.missing(1045) }
func (NotFound) OutboxCollection() *NotFound { return NotFound{}.missing(1046) }
func (NotFound) OutboxDocument() *NotFound   { return NotFound{}.missing(1047) }

func (NotFound) Domain(domainId string) *NotFound {
	return NotFound{}.missingWith(1010, "no hosted domain "+domainId)
}

func (NotFound) NotMemberOf(domainId string) *NotFound {
	return NotFound{}.missingWith(1037, "you are not a member of "+domainId)
}

func (NotFound) missing(number int) *NotFound {
	return &NotFound{ErrorCase{number, "not_found", "", NotFound{}.texts()[number]}}
}

func (NotFound) missingWith(number int, text string) *NotFound {
	return &NotFound{ErrorCase{number, "not_found", "", text}}
}

func (NotFound) texts() map[int]string {
	return map[int]string{
		1003: "not found",
		1015: "no such address",
		1017: "no such member in your domain",
		1021: "no such channel address",
		1032: "no such account",
		1038: "no such person",
		1039: "this person was deleted",
		1040: "not found",
		1045: "no such dead event",
		1046: "no such outbox collection",
		1047: "no such outbox document",
	}
}
