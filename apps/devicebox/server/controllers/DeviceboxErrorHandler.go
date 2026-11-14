import "mesail.dev/packages/apierrors"

// @ErrorHandler
type DeviceboxErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *DeviceboxErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return ApiError{Code: "invalid_request", Message: handler.message(failure, failure.Message), Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *DeviceboxErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return handler.answer("unauthenticated", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(403)
func (handler *DeviceboxErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return handler.answer("forbidden", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(404)
func (handler *DeviceboxErrorHandler) Missing(failure *NotFound) ApiError {
	return handler.answer("not_found", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(409)
func (handler *DeviceboxErrorHandler) Duplicate(failure *Duplicate) ApiError {
	return handler.answer("duplicate", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(413)
func (handler *DeviceboxErrorHandler) TooLarge(failure *TooLarge) ApiError {
	return handler.answer("too_large", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(429)
func (handler *DeviceboxErrorHandler) BoxFull(failure *BoxFull) ApiError {
	return handler.answer("box_full", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(503)
func (handler *DeviceboxErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return handler.answer(handler.unavailable(), failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(500)
func (handler *DeviceboxErrorHandler) Unexpected(failure error) ApiError {
	return handler.Unavailable(Unavailable{}.Unexpected())
}

func (handler *DeviceboxErrorHandler) answer(code string, failure apierrors.Numbered, text string) ApiError {
	return ApiError{Code: code, Message: handler.message(failure, text)}
}

func (handler *DeviceboxErrorHandler) message(failure apierrors.Numbered, text string) string {
	return apierrors.Message{}.Format(failure.Number(), text)
}

func (handler *DeviceboxErrorHandler) unavailable() string {
	return "unavailable"
}
