type CategoryPickerItem struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	DisplayName   string `json:"displayName"`
	Depth         int    `json:"depth"`
	ParentPath    string `json:"parentPath"`
	Role          string `json:"role"`
	SelectedState string `json:"selectedState"`
	Indent        string `json:"indent"`
	Check         string `json:"check"`
	AriaSelected  string `json:"ariaSelected"`
}
