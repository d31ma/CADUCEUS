import "mesail.dev/packages/webpush"

type PushDelivery struct {
	DomainId string          `json:"domainId"`
	MemberId string          `json:"memberId"`
	Message  webpush.Message `json:"message"`
}
