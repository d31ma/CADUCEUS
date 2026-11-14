package devicebox

type UnwiredDirectory struct{}

func (UnwiredDirectory) Owners(domainId string, deviceIds []string) (map[string]string, error) {
	return nil, Unavailable{}.DirectoryNotWired()
}
