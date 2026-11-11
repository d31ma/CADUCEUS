// @ErrorHandler
type InboundErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *InboundErrorHandler) Invalid(failure *InvalidEnvelope) ApiError {
	return ApiError{Code: "invalid_request", Message: failure.Reason, Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *InboundErrorHandler) Unavailable(failure *StorageUnavailable) ApiError {
	return ApiError{Code: "unavailable", Message: "Inbound storage is not available. Try again later."}
}
