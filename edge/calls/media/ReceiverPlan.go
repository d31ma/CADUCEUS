package media

type ReceiverPlan struct {
	Video   []FeedKey
	Screen  FeedKey
	Audio   []FeedKey
	Message SlotMessage
}
