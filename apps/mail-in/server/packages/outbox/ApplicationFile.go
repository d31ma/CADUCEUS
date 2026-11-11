package outbox

type ApplicationFile struct {
	Yon    ApplicationSection `json:"yon"`
	Outbox *Settings          `json:"outbox"`
}

func (file ApplicationFile) outbox() *Settings {
	if file.Yon.Outbox != nil {
		return file.Yon.Outbox
	}
	return file.Outbox
}
