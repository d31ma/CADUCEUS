import (
	"os"

	"mesail.dev/packages/mailin"
	"mesail.dev/packages/outbox"
)

// @Service
type WorkflowService struct {
	// @Needs
	dataSource DataSource
	pusher     outbox.Pusher
}

func (service *WorkflowService) Inbox() mailin.Inbox {
	return service.factory().Inbox(service.dataSource, service.page)
}

func (service *WorkflowService) Outbox() mailin.OutboxDesk {
	return service.factory().Outbox(service.dataSource, service.page)
}

func (service *WorkflowService) Messages() mailin.MessageDesk {
	return service.factory().Messages(service.dataSource, service.page)
}

func (service *WorkflowService) factory() mailin.DeskFactory {
	return mailin.DeskFactory{Environment: os.Getenv, Pusher: service.pusher}
}

func (service *WorkflowService) page(collection string, where map[string]any, cursor string, limit int) (mailin.Page, error) {
	window, err := service.dataSource.Scroll(collection, where, cursor, limit)
	return mailin.Page{Content: window.Content, NextCursor: window.NextCursor}, err
}
