type PresenceRequest struct {
	State    string `json:"state"`
	DndUntil string `json:"dndUntil,omitempty"`
}
