type PendingLinkWarning struct {
	Reason          string `json:"reason"`
	Target          string `json:"target"`
	DestinationHost string `json:"destinationHost"`
	DisplayedHost   string `json:"displayedHost"`
}
