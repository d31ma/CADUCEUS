package webpush

type SettingsDocument struct {
	Yon YonSection `json:"yon"`
}

func (document SettingsDocument) Keys() VapidKeys {
	vapid := document.Yon.Notify.Vapid
	return VapidKeys{PublicKey: vapid.Public.Key, PrivateKey: vapid.Private.Key, Subject: vapid.Subject}
}
