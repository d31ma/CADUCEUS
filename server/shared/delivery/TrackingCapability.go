// TrackingCapability is the content of one encrypted tracking token: an open
// or a click (with its target and link index) of one tracked message, for one
// recipient, until ExpiresAt (milliseconds).
type TrackingCapability struct {
	Type       string
	TrackingId string
	Recipient  string
	Url        string
	LinkIndex  int
	ExpiresAt  int64
}
