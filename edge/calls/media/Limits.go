package media

type Limits struct{}

func (Limits) People() int            { return 50 }
func (Limits) VideosPerReceiver() int { return 9 }
