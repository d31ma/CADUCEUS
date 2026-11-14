package devicebox

type TokenSource interface {
	Authorization(domainId string) (string, error)
}
