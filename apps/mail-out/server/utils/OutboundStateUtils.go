type OutboundStateUtils struct{}

func (OutboundStateUtils) Queued() string   { return "queued" }
func (OutboundStateUtils) Sending() string  { return "sending" }
func (OutboundStateUtils) Sent() string     { return "sent" }
func (OutboundStateUtils) Deferred() string { return "deferred" }
func (OutboundStateUtils) Bounced() string  { return "bounced" }
func (OutboundStateUtils) Expired() string  { return "expired" }
func (OutboundStateUtils) Failed() string   { return "failed" }
