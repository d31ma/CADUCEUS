import "mesail.dev/packages/devicebox"

// @RequestRoute("/api/v1/devicebox")
// @SignedIn
type EnvelopesController struct {
	// @Needs
	mailbox *MailboxService
}

// @PostRoute("/envelopes")
// @ResponseStatus(201)
func (controller *EnvelopesController) PostEnvelope(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request EnvelopePut,
) (devicebox.EnvelopeReceipt, error) {
	return controller.mailbox.Put(authorization, request)
}

// @GetRoute("/devices/{device}/envelopes")
func (controller *EnvelopesController) GetEnvelopes(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("device")
	device string,
	// @FromQuery("after")
	after string,
	// @FromQuery("limit")
	limit string,
) (devicebox.EnvelopePage, error) {
	return controller.mailbox.List(authorization, device, after, limit)
}

// @DeleteRoute("/devices/{device}/envelopes")
// @ResponseStatus(204)
func (controller *EnvelopesController) DeleteEnvelopes(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("device")
	device string,
	// @FromBody
	request EnvelopeAck,
) error {
	return controller.mailbox.Ack(authorization, device, request)
}
