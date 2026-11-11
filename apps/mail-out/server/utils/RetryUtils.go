import "time"

type RetryUtils struct{}

func (RetryUtils) largestShift() int { return 20 }

func (utils RetryUtils) Delay(attempts int, first time.Duration, largest time.Duration) time.Duration {
	shift := min(max(attempts-1, 0), utils.largestShift())
	return min(first*time.Duration(1<<shift), largest)
}
