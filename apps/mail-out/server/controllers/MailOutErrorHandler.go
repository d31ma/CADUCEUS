import (
	"mesail.dev/packages/apierrors"
)

// @ErrorHandler
type MailOutErrorHandler struct{}

// @HandlesError
// @ResponseStatus(400)
func (handler *MailOutErrorHandler) Rejected(failure *RejectedEvent) ApiError {
	return ApiError{Code: "invalid_request", Message: handler.message(failure, failure.Reason)}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *MailOutErrorHandler) Forbidden(failure *ForbiddenCaller) ApiError {
	return ApiError{Code: "forbidden", Message: handler.message(failure, "Only a caller with the Service role may deliver events.")}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *MailOutErrorHandler) Unknown(failure *UnknownDomain) ApiError {
	return ApiError{Code: "not_found", Message: handler.message(failure, "Not found.")}
}

// @HandlesError
// @ResponseStatus(500)
func (handler *MailOutErrorHandler) Unexpected(failure error) ApiError {
	unexpected := &UnexpectedError{}
	return ApiError{Code: "unavailable", Message: handler.message(unexpected, unexpected.Error())}
}

func (handler *MailOutErrorHandler) message(failure apierrors.Numbered, text string) string {
	return apierrors.Message{}.Format(failure.Number(), text)
}
