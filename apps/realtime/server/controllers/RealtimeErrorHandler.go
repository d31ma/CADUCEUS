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
// @ResponseStatus(404)
func (handler *RealtimeErrorHandler) Missing(failure *NotFound) ApiError {
	return ApiError{Code: "not_found", Message: "Nothing was found."}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *RealtimeErrorHandler) Storage(failure *StorageUnavailable) ApiError {
	return ApiError{Code: "unavailable", Message: "Realtime storage is not available. Try again later."}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *RealtimeErrorHandler) Gateway(failure *GatewayUnavailable) ApiError {
	return ApiError{Code: "unavailable", Message: "The connection gateway is not available. Try again later."}
}
