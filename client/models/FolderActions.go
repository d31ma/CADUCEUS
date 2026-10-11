type FolderActions struct {
	CreateChild bool `json:"createChild"`
	Rename      bool `json:"rename"`
	Delete      bool `json:"delete"`
	Export      bool `json:"export"`
}
