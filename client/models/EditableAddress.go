// EditableAddress is one address block of the contact editor.
type EditableAddress struct {
	Key             string `json:"key"`
	Type            string `json:"type"`
	StreetAddress   string `json:"streetAddress"`
	ExtendedAddress string `json:"extendedAddress"`
	City            string `json:"city"`
	Region          string `json:"region"`
	PostalCode      string `json:"postalCode"`
	Country         string `json:"country"`
}
