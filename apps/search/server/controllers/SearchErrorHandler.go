import "mesail.dev/packages/apierrors"

// @ErrorHandler
type SearchErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *SearchErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return ApiError{Code: "invalid_request", Message: handler.message(failure, failure.Message), Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *SearchErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: handler.message(failure, "Sign in again.")}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *SearchErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return ApiError{Code: "forbidden", Message: handler.message(failure, failure.Message)}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *SearchErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return ApiError{Code: handler.unavailable(), Message: handler.message(failure, "Search is not available. Try again later.")}
}

// @HandlesError
// @ResponseStatus(500)
func (handler *SearchErrorHandler) Unexpected(failure error) ApiError {
	return ApiError{Code: handler.unavailable(), Message: handler.message(&UnexpectedFailure{}, "Something went wrong.")}
}

func (handler *SearchErrorHandler) message(failure apierrors.Numbered, text string) string {
	return apierrors.Message{}.Format(failure.Number(), text)
}

func (handler *SearchErrorHandler) unavailable() string {
	return "unavailable"
}
