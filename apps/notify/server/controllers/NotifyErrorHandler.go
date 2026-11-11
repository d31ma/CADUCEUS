// @ErrorHandler
type NotifyErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *NotifyErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return ApiError{Code: "invalid_request", Message: failure.Reason, Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *NotifyErrorHandler) Refused(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: "Sign in again."}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *NotifyErrorHandler) Forbidden(failure *NotService) ApiError {
	return ApiError{Code: "forbidden", Message: "Only another Mesail service may call this route."}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *NotifyErrorHandler) Missing(failure *NotFound) ApiError {
	return ApiError{Code: "not_found", Message: "The " + failure.What + " was not found."}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *NotifyErrorHandler) Unavailable(failure *StorageUnavailable) ApiError {
	return ApiError{Code: handler.unavailableCode(), Message: "Notification storage is not available. Try again later."}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *NotifyErrorHandler) PushUnavailable(failure *PushNotConfigured) ApiError {
	return ApiError{Code: handler.unavailableCode(), Message: "Web push is not configured on this server."}
}

func (handler *NotifyErrorHandler) unavailableCode() string {
	return "unavailable"
}
