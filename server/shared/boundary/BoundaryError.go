// BoundaryError carries the stable code of a failed FYLO or authentication
// boundary operation, such as ENOTFOUND or EPOLICY.
type BoundaryError struct {
	Code    string
	Message string
}

func (failure *BoundaryError) Error() string {
	return failure.Message
}
