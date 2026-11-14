package media

type FeedKey struct {
	DeviceId string
	Kind     FeedKind
	Rid      string
}

func (key FeedKey) Empty() bool {
	return key.DeviceId == ""
}
