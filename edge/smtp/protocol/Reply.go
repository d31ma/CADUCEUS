package protocol

import (
	"strconv"
	"strings"
)

type Reply struct {
	Code  int
	Lines []string
}

func (reply Reply) IsEmpty() bool {
	return len(reply.Lines) == 0
}

func (reply Reply) IsPermanentError() bool {
	return reply.Code >= reply.firstPermanentCode()
}

func (reply Reply) Wire() string {
	return reply.render()
}

func (reply Reply) render() string {
	var text strings.Builder
	for index, line := range reply.Lines {
		text.WriteString(strconv.Itoa(reply.Code) + reply.separator(index) + line + Limits{}.LineEnding())
	}
	return text.String()
}

func (reply Reply) separator(index int) string {
	if index < len(reply.Lines)-1 {
		return "-"
	}
	return " "
}

func (Reply) firstPermanentCode() int {
	return 500
}
