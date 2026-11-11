package addresses

type Reservation struct {
	LocalPart string   `json:"localPart"`
	Owner     string   `json:"owner"`
	PersonId  string   `json:"personId,omitempty"`
	DomainIds []string `json:"domainIds,omitempty"`
}
