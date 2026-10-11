// ComposeLimits are the composer's limits: ten files, 32 KB of attachments and
// 64 KiB in one request.
type ComposeLimits struct {
	Files       int
	Attachments int
	Request     int
}
