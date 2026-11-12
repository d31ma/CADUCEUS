import (
	"mesail.dev/packages/apierrors"
)

// @ErrorHandler
type InboundErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *InboundErrorHandler) Invalid(failure *InvalidEnvelope) ApiError {
	return ApiError{Code: "invalid_request", Message: handler.message(failure, failure.Reason), Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *InboundErrorHandler) Refused(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: handler.message(failure, "The access token was refused.")}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *InboundErrorHandler) Missing(failure *NotFound) ApiError {
	return ApiError{Code: "not_found", Message: handler.message(failure, failure.Message)}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *InboundErrorHandler) Unavailable(failure *StorageUnavailable) ApiError {
	return ApiError{Code: handler.unavailable(), Message: handler.message(failure, "Inbound storage is not available. Try again later.")}
}

// @HandlesError
// @ResponseStatus(500)
func (handler *InboundErrorHandler) Unexpected(failure error) ApiError {
	unexpected := &UnexpectedError{}
	return ApiError{Code: handler.unavailable(), Message: handler.message(unexpected, unexpected.Error())}
}

func (handler *InboundErrorHandler) message(failure apierrors.Numbered, text string) string {
	return apierrors.Message{}.Format(failure.Number(), text)
}

func (handler *InboundErrorHandler) unavailable() string {
	return "unavailable"
}
