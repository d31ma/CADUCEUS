import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// @Delegate
// @NeedsAll()
type DirectoryDelegate struct {
	settings *SettingsDelegate
	client   *ServiceClientDelegate
}

func (DirectoryDelegate) route() string { return "/api/v1/directory/domains/" }

func (directory *DirectoryDelegate) Domain(domainId string, realm string, authorize func(string) (string, error)) (Domain, bool, error) {
	address := strings.TrimRight(directory.settings.Settings().DirectoryUrl, "/") + directory.route() + url.PathEscape(strings.ToLower(domainId))
	domain := Domain{}
	status, err := directory.client.Get(address, realm, authorize, &domain)
	return directory.answer(domain, status, err)
}

func (DirectoryDelegate) answer(domain Domain, status int, failure error) (Domain, bool, error) {
	switch {
	case failure != nil:
		return Domain{}, false, failure
	case status == http.StatusOK:
		return domain, true, nil
	case status == http.StatusNotFound:
		return Domain{}, false, nil
	default:
		return Domain{}, false, fmt.Errorf("directory answered %d", status)
	}
}
