import (
	"crypto/rand"
	"time"

	"mesail.dev/packages/adapters"
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

// @Service
type CoreService struct {
	// @Needs
	storage *StorageService
	// @Needs
	settings    *SettingsService
	fixedStores *core.Stores
	fixedPorts  *core.Ports
}

func (service *CoreService) Stores() core.Stores {
	return service.storesOr(service.fixedStores)
}

func (service *CoreService) Ports() core.Ports {
	return service.portsOr(service.fixedPorts)
}

func (service *CoreService) storesOr(fixed *core.Stores) core.Stores {
	if fixed != nil {
		return *fixed
	}
	return service.storage.Storage().Stores()
}

func (service *CoreService) portsOr(fixed *core.Ports) core.Ports {
	if fixed != nil {
		return *fixed
	}
	return service.builtPorts()
}

func (service *CoreService) builtPorts() core.Ports {
	urls := service.settings.Settings()
	tokens := oidc.ServiceTokensFactory{}.Configured()
	call := adapters.NewJsonCall(adapters.JsonCall{}.DefaultTimeout())
	outboxSettings, _ := service.settings.Outbox()
	return core.Ports{
		Directory: adapters.NewDirectoryClient(urls.DirectoryUrl, tokens, call),
		Renderer:  adapters.RichTextRenderer{},
		Inbound:   adapters.NewMailInClient(urls.MailInUrl, tokens, call),
		Stager:    core.NewStager(outboxSettings, time.Now, rand.Reader),
		Now:       time.Now,
	}
}
