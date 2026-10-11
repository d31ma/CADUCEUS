// AuthLimit counts the attempts of one /auth/* route per key: the route's
// name, its first body field (the address or transaction) and the caller's
// network address.
type AuthLimit struct {
	Name          string
	Attempts      int
	WindowMinutes int64
}
