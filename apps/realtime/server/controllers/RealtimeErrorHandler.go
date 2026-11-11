// @ErrorHandler
type RealtimeErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *RealtimeErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return ApiError{Code: "invalid_request", Message: failure.Reason, Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *RealtimeErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: "Sign in again."}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *RealtimeErrorHandler) Refused(failure *Forbidden) ApiError {
	return ApiError{Code: "forbidden", Message: "This caller may not use this route."}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *RealtimeErrorHandler) Missing(failure *NotFound) ApiError {
	return ApiError{Code: "not_found", Message: "Nothing was found."}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *RealtimeErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return ApiError{Code: "unavailable", Message: "Realtime is not available right now. Try again later."}
}
