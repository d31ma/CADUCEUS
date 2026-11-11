import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"mesail.dev/packages/events"
)

// @Delegate
// @NeedsAll()
type MessageSourceDelegate struct {
	settings *SettingsDelegate
	client   *ServiceClientDelegate
}

func (MessageSourceDelegate) route() string { return "/messages/" }

func (MessageSourceDelegate) suffix() string { return "/source" }

func (MessageSourceDelegate) apiPrefix() string { return "/api/v1/" }

func (sources *MessageSourceDelegate) Source(item OutboundItem, authorize func(string) (string, error)) (MessageSource, error) {
	base, known := sources.bases()[item.Producer]
	address := strings.TrimRight(base, "/") + sources.apiPrefix() + item.Producer + sources.route() + url.PathEscape(item.MessageId) + sources.suffix()
	return sources.fetch(address, item, known, authorize)
}

func (sources *MessageSourceDelegate) Knows(producer string) bool {
	_, known := sources.bases()[producer]
	return known
}

func (sources *MessageSourceDelegate) bases() map[string]string {
	settings := sources.settings.Settings()
	services := events.Services{}
	return map[string]string{services.Messaging(): settings.MessagingUrl, services.Notify(): settings.NotifyUrl}
}

func (sources *MessageSourceDelegate) fetch(address string, item OutboundItem, known bool, authorize func(string) (string, error)) (MessageSource, error) {
	if !known {
		return MessageSource{}, fmt.Errorf("no message source is known for the producer %q", item.Producer)
	}
	source := MessageSource{}
	status, err := sources.client.Get(address, item.DomainId, authorize, &source)
	return source, sources.checked(status, err)
}

func (MessageSourceDelegate) checked(status int, failure error) error {
	if failure == nil && status != http.StatusOK {
		return fmt.Errorf("the producer answered %d for the message source", status)
	}
	return failure
}
