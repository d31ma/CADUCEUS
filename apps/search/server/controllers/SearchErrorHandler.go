// @ErrorHandler
type SearchErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *SearchErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return ApiError{Code: "invalid_request", Message: failure.Message, Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *SearchErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: "Sign in again."}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *SearchErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return ApiError{Code: "forbidden", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *SearchErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return ApiError{Code: "unavailable", Message: "Search is not available. Try again later."}
}
