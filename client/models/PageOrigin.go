// PageOrigin (retained) is the page's scheme, host and port, which Go cannot
// read: a copied sharing link starts with it.
type PageOrigin struct {
	Origin string `json:"origin"`
}
