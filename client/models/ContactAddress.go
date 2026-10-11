type ContactAddress struct {
	Type            string `json:"type"`
	StreetAddress   string `json:"streetAddress"`
	ExtendedAddress string `json:"extendedAddress"`
	City            string `json:"city"`
	Region          string `json:"region"`
	PostalCode      string `json:"postalCode"`
	Country         string `json:"country"`
}
