package smtpclient

type Reply struct {
	Code     int
	Enhanced string
	Text     string
}

func (reply Reply) Permanent() bool {
	return reply.Code >= reply.permanentStart()
}

func (reply Reply) Temporary() bool {
	return reply.temporary()
}

func (Reply) permanentStart() int { return 500 }

func (Reply) temporaryStart() int { return 400 }

func (reply Reply) temporary() bool {
	return reply.Code == 0 || (reply.Code >= reply.temporaryStart() && reply.Code < reply.permanentStart())
}
