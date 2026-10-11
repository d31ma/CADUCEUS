type Category struct {
	Path        string  `json:"path"`
	Kind        string  `json:"kind"`
	DisplayName string  `json:"displayName"`
	Depth       float64 `json:"depth"`
	ParentPath  string  `json:"parentPath"`
	Role        string  `json:"role"`
}
