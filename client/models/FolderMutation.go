type FolderMutation struct {
	Revision         int      `json:"revision"`
	IdempotencyKey   string   `json:"idempotencyKey"`
	Kind             string   `json:"kind"`
	MessageStableIds []string `json:"messageStableIds"`
	FolderStableID   string   `json:"folderStableId"`
	ParentStableID   string   `json:"parentStableId"`
	FolderName       string   `json:"folderName"`
}
