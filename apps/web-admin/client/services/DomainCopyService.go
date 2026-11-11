// @Service
type DomainCopyService struct {
	Copied  string
	Notice  string
	Problem string
	// @Needs
	clipboard Clipboard
}

// @OnEvent({"condition":"value.view == 'domain'","replay":true})
func (copying *DomainCopyService) Opened(opened AdminViewOpened) {
	copying.Copied = ""
	copying.Notice = ""
	copying.Problem = ""
}

func (copying *DomainCopyService) Copy(text string) {
	failure := copying.clipboard.WriteText(text)
	copying.Copied = copying.copiedWhen(failure, text)
	copying.Notice = copying.noticeWhen(failure, text)
	copying.Problem = copying.problemWhen(failure)
}

func (copying *DomainCopyService) copiedWhen(failure error, text string) string {
	if failure != nil {
		return ""
	}
	return text
}

func (copying *DomainCopyService) noticeWhen(failure error, text string) string {
	if failure != nil {
		return ""
	}
	return "Copied " + text + "."
}

func (copying *DomainCopyService) problemWhen(failure error) string {
	if failure != nil {
		return "Copying did not work. Select the text and copy it yourself."
	}
	return ""
}
