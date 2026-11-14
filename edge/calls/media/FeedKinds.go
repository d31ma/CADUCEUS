package media

type FeedKinds struct{}

func (FeedKinds) Microphone() FeedKind { return "microphone" }
func (FeedKinds) Camera() FeedKind     { return "camera" }
func (FeedKinds) Screen() FeedKind     { return "screen" }
