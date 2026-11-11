import "strconv"

type PresenceUnavailable struct {
	Status int
}

func (failure *PresenceUnavailable) Error() string {
	return "the presence service answered with status " + strconv.Itoa(failure.Status)
}
