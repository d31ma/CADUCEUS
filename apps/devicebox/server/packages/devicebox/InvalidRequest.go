package devicebox

type InvalidRequest struct {
	Message     string
	Field       string
	ErrorNumber int
}

func (InvalidRequest) EnvelopeId() *InvalidRequest {
	return &InvalidRequest{Message: "The envelope id must be a UUID.", Field: "envelopeId", ErrorNumber: 6003}
}

func (InvalidRequest) SenderDevice() *InvalidRequest {
	return &InvalidRequest{Message: "The sending device id is not valid.", Field: "senderDeviceId", ErrorNumber: 6004}
}

func (InvalidRequest) DeviceCount() *InvalidRequest {
	return &InvalidRequest{Message: "Name 1 to 200 receiving devices.", Field: "toDeviceIds", ErrorNumber: 6005}
}

func (InvalidRequest) DeviceId() *InvalidRequest {
	return &InvalidRequest{Message: "A receiving device id is empty, too long or repeated.", Field: "toDeviceIds", ErrorNumber: 6006}
}

func (InvalidRequest) Kind() *InvalidRequest {
	return &InvalidRequest{Message: "The kind must be message, group or linking.", Field: "kind", ErrorNumber: 6007}
}

func (InvalidRequest) Ciphertext() *InvalidRequest {
	return &InvalidRequest{Message: "The ciphertext must be base64url text that is not empty.", Field: "ciphertext", ErrorNumber: 6008}
}

func (InvalidRequest) After() *InvalidRequest {
	return &InvalidRequest{Message: "The page cursor is not valid.", Field: "after", ErrorNumber: 6015}
}

func (InvalidRequest) Limit() *InvalidRequest {
	return &InvalidRequest{Message: "The page size must be 1 to 100.", Field: "limit", ErrorNumber: 6016}
}

func (InvalidRequest) AckCount() *InvalidRequest {
	return &InvalidRequest{Message: "Acknowledge 1 to 10 envelopes at a time.", Field: "envelopeIds", ErrorNumber: 6017}
}

func (InvalidRequest) AckId() *InvalidRequest {
	return &InvalidRequest{Message: "Every acknowledged envelope id must be a UUID.", Field: "envelopeIds", ErrorNumber: 6018}
}

func (failure *InvalidRequest) Error() string {
	return failure.Message
}

func (failure *InvalidRequest) Number() int {
	return failure.ErrorNumber
}
