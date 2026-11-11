package channels

import "strconv"

type DirectoryFailed struct {
	Action string
	Status int
}

func (failure DirectoryFailed) Error() string {
	return "the directory refused " + failure.Action + " with status " + strconv.Itoa(failure.Status)
}
