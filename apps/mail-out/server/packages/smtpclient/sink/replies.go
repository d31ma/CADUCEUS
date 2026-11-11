package sink

type replies struct{}

func (replies) Ready() reply           { return reply{220, "127.0.0.1 ESMTP sink ready"} }
func (replies) TlsReady() reply        { return reply{220, "2.0.0 Ready to start TLS"} }
func (replies) Closing() reply         { return reply{221, "2.0.0 Bye"} }
func (replies) AuthOkay() reply        { return reply{235, "2.7.0 Authentication successful"} }
func (replies) Okay() reply            { return reply{250, "2.0.0 OK"} }
func (replies) Queued() reply          { return reply{250, "2.0.0 OK queued"} }
func (replies) StartData() reply       { return reply{354, "End data with <CR><LF>.<CR><LF>"} }
func (replies) TlsUnavailable() reply  { return reply{454, "4.7.0 TLS not available"} }
func (replies) Unknown() reply         { return reply{500, "5.5.2 Command not recognized"} }
func (replies) BadSequence() reply     { return reply{503, "5.5.1 Bad sequence of commands"} }
func (replies) AuthUnavailable() reply { return reply{503, "5.5.1 Authentication not available"} }
func (replies) AuthRequired() reply    { return reply{530, "5.7.0 Authentication required"} }
func (replies) AuthFailed() reply      { return reply{535, "5.7.8 Authentication credentials invalid"} }
func (replies) TooLarge() reply        { return reply{552, "5.3.4 Message too big"} }
