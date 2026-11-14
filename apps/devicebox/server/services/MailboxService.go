import (
	"time"

	"mesail.dev/packages/devicebox"
	"mesail.dev/packages/outbox"
)

// @Service
type MailboxService struct {
	// @Needs
	dataSource DataSource
	// @Needs
	failures *ErrorMappingService
	storage  devicebox.Storage
	pusher   outbox.Pusher
	devices  devicebox.DeviceDirectory
	clock    devicebox.Clock
}

func (service *MailboxService) Put(authorization string, request EnvelopePut) (devicebox.EnvelopeReceipt, error) {
	put := devicebox.PutRequest{EnvelopeId: request.EnvelopeId, SenderDeviceId: request.SenderDeviceId, ToDeviceIds: request.ToDeviceIds, Kind: request.Kind, Ciphertext: request.Ciphertext}
	receipt, err := service.assembly().Mailbox().Put(authorization, put)
	return receipt, service.failures.Mailbox(err)
}

func (service *MailboxService) List(authorization string, device string, after string, limit string) (devicebox.EnvelopePage, error) {
	page, err := service.assembly().Mailbox().List(authorization, device, after, limit)
	return page, service.failures.Mailbox(err)
}

func (service *MailboxService) Ack(authorization string, device string, request EnvelopeAck) error {
	err := service.assembly().Mailbox().Ack(authorization, device, request.EnvelopeIds)
	return service.failures.Mailbox(err)
}

func (service *MailboxService) Expire(authorization string) (devicebox.ExpireReport, error) {
	report, err := service.assembly().Expiry().Run(authorization)
	return report, service.failures.Mailbox(err)
}

func (service *MailboxService) assembly() devicebox.Assembly {
	return devicebox.Assembly{Source: service.source(), Pusher: service.pusher, Directory: service.devices, Now: service.now()}
}

func (service *MailboxService) now() func() time.Time {
	if service.clock == nil {
		return nil
	}
	return service.clock.Now
}

func (service *MailboxService) source() devicebox.Storage {
	if service.storage != nil {
		return service.storage
	}
	return service.dataSource
}
