import "mesail.dev/packages/apierrors"

// @ErrorHandler
type NotifyErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *NotifyErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(401)
func (handler *NotifyErrorHandler) Refused(failure *Unauthenticated) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(403)
func (handler *NotifyErrorHandler) Forbidden(failure *NotService) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(404)
func (handler *NotifyErrorHandler) Missing(failure *NotFound) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(503)
func (handler *NotifyErrorHandler) Unavailable(failure *StorageUnavailable) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(503)
func (handler *NotifyErrorHandler) PushUnavailable(failure *PushNotConfigured) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(500)
func (handler *NotifyErrorHandler) Unexpected(failure error) ApiError {
	return handler.answer(UnexpectedFailure{}.Unexpected().ErrorCase)
}

func (handler *NotifyErrorHandler) answer(failure ErrorCase) ApiError {
	return ApiError{Code: failure.Code(), Message: apierrors.Message{}.Format(failure.Number(), failure.Error()), Field: failure.Field()}
}
