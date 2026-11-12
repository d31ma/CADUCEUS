import "mesail.dev/packages/apierrors"

// @ErrorHandler
type AttachmentsErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *AttachmentsErrorHandler) Invalid(failure *InvalidUpload) ApiError {
	return ApiError{Code: "invalid_request", Message: handler.message(failure, failure.Reason), Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *AttachmentsErrorHandler) Refused(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: handler.message(failure, "Sign in again.")}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *AttachmentsErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return ApiError{Code: "forbidden", Message: handler.message(failure, failure.Message)}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *AttachmentsErrorHandler) Missing(failure *AttachmentNotFound) ApiError {
	return ApiError{Code: "not_found", Message: handler.message(failure, "The attachment was not found.")}
}

// @HandlesError
// @ResponseStatus(409)
func (handler *AttachmentsErrorHandler) Conflicting(failure *AttachmentConflict) ApiError {
	return ApiError{Code: "conflict", Message: handler.message(failure, failure.Reason), Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(413)
func (handler *AttachmentsErrorHandler) Large(failure *AttachmentTooLarge) ApiError {
	return ApiError{Code: "too_large", Message: handler.message(failure, failure.Error()), Field: "size"}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *AttachmentsErrorHandler) Unavailable(failure *StorageUnavailable) ApiError {
	return ApiError{Code: handler.unavailable(), Message: handler.message(failure, "Attachment storage is not available. Try again later.")}
}

// @HandlesError
// @ResponseStatus(500)
func (handler *AttachmentsErrorHandler) Unexpected(failure error) ApiError {
	return ApiError{Code: handler.unavailable(), Message: handler.message(&UnexpectedFailure{}, "Something went wrong.")}
}

func (handler *AttachmentsErrorHandler) message(failure apierrors.Numbered, text string) string {
	return apierrors.Message{}.Format(failure.Number(), text)
}

func (handler *AttachmentsErrorHandler) unavailable() string {
	return "unavailable"
}
