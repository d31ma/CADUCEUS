import "fmt"

type AttachmentTooLarge struct {
	Limit int64
}

func (failure *AttachmentTooLarge) Error() string {
	return fmt.Sprintf("the file is larger than %d bytes", failure.Limit)
}

func (failure *AttachmentTooLarge) Number() int {
	return 3512
}
