// PickerMessage is what the category picker needs to know about one selected
// message.
type PickerMessage struct {
	ID     string   `json:"id"`
	Domain string   `json:"domain"`
	Folder string   `json:"folder"`
	Labels []string `json:"labels"`
}
