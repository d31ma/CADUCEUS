// Open the move or label picker for these messages.
type CategoryPickerRequested struct {
	Mode     string          `json:"mode"`
	Messages []PickerMessage `json:"messages"`
}
