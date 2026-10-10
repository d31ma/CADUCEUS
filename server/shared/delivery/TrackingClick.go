// TrackingClick is one verified click on a tracked link: the capability's
// tracking id and recipient, and the link's index and target.
type TrackingClick struct {
	TrackingId string
	Recipient  string
	Url        string
	LinkIndex  float64
}
