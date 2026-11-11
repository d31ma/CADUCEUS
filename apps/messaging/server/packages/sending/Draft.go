package sending

import "mesail.dev/packages/core"

type Draft struct {
	Request     core.SendRequest
	SendKey     string
	Placement   Placement
	Thread      Thread
	Rows        []core.DeliveryRecord
	IsEmail     bool
	Email       core.EmailMeta
	Rich        core.RichText
	Mentions    []string
	Attachments []core.AttachmentRef
}
