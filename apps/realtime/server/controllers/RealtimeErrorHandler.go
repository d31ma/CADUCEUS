import "mesail.dev/packages/apierrors"

// @ErrorHandler
type RealtimeErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *RealtimeErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return ApiError{Code: "invalid_request", Message: handler.message(failure, failure.Message), Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *RealtimeErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return handler.answer("unauthenticated", failure, "Sign in again.")
}

// @HandlesError
// @ResponseStatus(403)
func (handler *RealtimeErrorHandler) Refused(failure *Forbidden) ApiError {
	return handler.answer("forbidden", failure, "This caller may not use this route.")
}

// @HandlesError
// @ResponseStatus(404)
func (handler *RealtimeErrorHandler) Missing(failure *NotFound) ApiError {
	return handler.answer("not_found", failure, "Nothing was found.")
}

// @HandlesError
// @ResponseStatus(503)
func (handler *RealtimeErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return handler.answer("unavailable", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(500)
func (handler *RealtimeErrorHandler) Unexpected(failure error) ApiError {
	return handler.Unavailable(Unavailable{}.Unexpected())
}

func (handler *RealtimeErrorHandler) answer(code string, failure apierrors.Numbered, text string) ApiError {
	return ApiError{Code: code, Message: handler.message(failure, text)}
}

func (handler *RealtimeErrorHandler) message(failure apierrors.Numbered, text string) string {
	return apierrors.Message{}.Format(failure.Number(), text)
}
