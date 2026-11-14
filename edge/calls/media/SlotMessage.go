package media

type SlotMessage struct {
	Type   string   `json:"type"`
	Video  []string `json:"video"`
	Screen string   `json:"screen"`
	Audio  []string `json:"audio"`
	Order  []string `json:"order"`
}
