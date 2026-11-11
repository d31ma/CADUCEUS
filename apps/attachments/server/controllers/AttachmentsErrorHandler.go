// @ErrorHandler
type AttachmentsErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *AttachmentsErrorHandler) Invalid(failure *InvalidUpload) ApiError {
	return ApiError{Code: "invalid_request", Message: failure.Reason, Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *AttachmentsErrorHandler) Refused(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: "Sign in again."}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *AttachmentsErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return ApiError{Code: "forbidden", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *AttachmentsErrorHandler) Missing(failure *AttachmentNotFound) ApiError {
	return ApiError{Code: "not_found", Message: "The attachment was not found."}
}

// @HandlesError
// @ResponseStatus(409)
func (handler *AttachmentsErrorHandler) Conflicting(failure *AttachmentConflict) ApiError {
	return ApiError{Code: "conflict", Message: failure.Reason, Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(413)
func (handler *AttachmentsErrorHandler) Large(failure *AttachmentTooLarge) ApiError {
	return ApiError{Code: "too_large", Message: failure.Error(), Field: "size"}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *AttachmentsErrorHandler) Unavailable(failure *StorageUnavailable) ApiError {
	return ApiError{Code: "unavailable", Message: "Attachment storage is not available. Try again later."}
}
