package channels

type InvalidRequest struct {
	Message     string
	Field       string
	ErrorNumber int
}

func (InvalidRequest) BadName() *InvalidRequest {
	return &InvalidRequest{Message: "a channel name has 1 to 80 lowercase letters, digits, - or _", Field: "name", ErrorNumber: 2003}
}

func (InvalidRequest) BadVisibility() *InvalidRequest {
	return &InvalidRequest{Message: "visibility is public or private", Field: "visibility", ErrorNumber: 2004}
}

func (InvalidRequest) TopicTooLong() *InvalidRequest {
	return &InvalidRequest{Message: "a topic has at most 250 characters", Field: "topic", ErrorNumber: 2005}
}

func (InvalidRequest) BadInviteSize(field string) *InvalidRequest {
	return &InvalidRequest{Message: "invite 1 to 100 members at a time", Field: field, ErrorNumber: 2009}
}

func (InvalidRequest) EmptyMemberId(field string) *InvalidRequest {
	return &InvalidRequest{Message: "a member id is empty", Field: field, ErrorNumber: 2010}
}

func (InvalidRequest) BadAddress() *InvalidRequest {
	return &InvalidRequest{Message: "a channel email address is local-part@ the channel's own domain", Field: "emailAddress", ErrorNumber: 2016}
}

func (InvalidRequest) AddressOutsideDomain(message string) *InvalidRequest {
	return &InvalidRequest{Message: message, Field: "emailAddress", ErrorNumber: 2017}
}

func (failure *InvalidRequest) Error() string {
	return failure.Message
}

func (failure *InvalidRequest) Number() int {
	return failure.ErrorNumber
}
