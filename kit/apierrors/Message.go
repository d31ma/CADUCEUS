package apierrors

import "fmt"

type Message struct{}

func (Message) Format(number int, text string) string {
	return fmt.Sprintf("[MS%04d]: %s", number, text)
}
