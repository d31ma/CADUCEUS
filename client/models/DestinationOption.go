// DestinationOption is one address book a new contact can be saved to.
type DestinationOption struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Selected bool   `json:"selected"`
}
