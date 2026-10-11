// CategoryCatalogChanged carries the folder and label catalog of every
// mailbox the person may change.
type CategoryCatalogChanged struct {
	Accounts []CategoryAccount `json:"accounts"`
}
