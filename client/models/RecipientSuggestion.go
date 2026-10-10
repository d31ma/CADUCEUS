type RecipientSuggestion struct {
	ContactID        string  `json:"contactId"`
	Name             string  `json:"name"`
	Email            string  `json:"email"`
	Source           string  `json:"source"`
	InteractionCount float64 `json:"interactionCount"`
	LastContactedAt  string  `json:"lastContactedAt"`
}
