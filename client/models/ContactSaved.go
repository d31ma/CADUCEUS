// ContactSaved names the contact the editor saved. Pending is true while its
// CardDAV change waits for the server.
type ContactSaved struct {
	ID      string `json:"id"`
	Pending bool   `json:"pending"`
}
