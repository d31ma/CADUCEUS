import "mesail.dev/packages/apierrors"

// @ErrorHandler
type ChannelsErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *ChannelsErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return ApiError{Code: "invalid_request", Message: handler.message(failure, failure.Message), Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *ChannelsErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return handler.answer("unauthenticated", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(403)
func (handler *ChannelsErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return handler.answer("forbidden", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(404)
func (handler *ChannelsErrorHandler) Missing(failure *NotFound) ApiError {
	return handler.answer("not_found", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(409)
func (handler *ChannelsErrorHandler) Conflict(failure *Conflict) ApiError {
	return handler.answer("conflict", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(409)
func (handler *ChannelsErrorHandler) Duplicate(failure *Duplicate) ApiError {
	return handler.answer("duplicate", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(503)
func (handler *ChannelsErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return handler.answer(handler.unavailable(), failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(500)
func (handler *ChannelsErrorHandler) Unexpected(failure error) ApiError {
	return handler.Unavailable(Unavailable{}.Unexpected())
}

func (handler *ChannelsErrorHandler) answer(code string, failure apierrors.Numbered, text string) ApiError {
	return ApiError{Code: code, Message: handler.message(failure, text)}
}

func (handler *ChannelsErrorHandler) message(failure apierrors.Numbered, text string) string {
	return apierrors.Message{}.Format(failure.Number(), text)
}

func (handler *ChannelsErrorHandler) unavailable() string {
	return "unavailable"
}
