type MessageEmailView struct {
	IsEmail          bool   `json:"isEmail"`
	Subject          string `json:"subject"`
	From             string `json:"from"`
	To               string `json:"to"`
	Cc               string `json:"cc"`
	SenderCheck      string `json:"senderCheck"`
	SenderCheckLabel string `json:"senderCheckLabel"`
	SenderCheckInfo  string `json:"senderCheckInfo"`
	HasQuoted        bool   `json:"hasQuoted"`
	BlockedImages    int    `json:"blockedImages"`
	BlockedLabel     string `json:"blockedLabel"`
	ImagesAllowed    bool   `json:"imagesAllowed"`
}
