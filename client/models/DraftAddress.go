// DraftAddress leaves out the parts that were left empty, as the editor did.
type DraftAddress struct {
	Type            string `json:"type,omitempty"`
	StreetAddress   string `json:"streetAddress,omitempty"`
	ExtendedAddress string `json:"extendedAddress,omitempty"`
	City            string `json:"city,omitempty"`
	Region          string `json:"region,omitempty"`
	PostalCode      string `json:"postalCode,omitempty"`
	Country         string `json:"country,omitempty"`
}
