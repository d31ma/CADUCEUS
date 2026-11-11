// @ErrorHandler
type ChannelsErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *ChannelsErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return ApiError{Code: "invalid_request", Message: failure.Message, Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *ChannelsErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *ChannelsErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return ApiError{Code: "forbidden", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *ChannelsErrorHandler) Missing(failure *NotFound) ApiError {
	return ApiError{Code: "not_found", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(409)
func (handler *ChannelsErrorHandler) Conflict(failure *Conflict) ApiError {
	return ApiError{Code: "conflict", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(409)
func (handler *ChannelsErrorHandler) Duplicate(failure *Duplicate) ApiError {
	return ApiError{Code: "duplicate", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *ChannelsErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return ApiError{Code: "unavailable", Message: failure.Message}
}
