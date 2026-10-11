// ValueEdit is one change to an email, phone or website row of the contact
// editor: Kind is "emails", "phones" or "urls" and Field is "type" or "value".
type ValueEdit struct {
	Kind  string `json:"kind"`
	Key   string `json:"key"`
	Field string `json:"field"`
	Value string `json:"value"`
}
