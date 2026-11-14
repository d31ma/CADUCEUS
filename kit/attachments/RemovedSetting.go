package attachments

type RemovedSetting struct {
	Name  string
	Value string
}

func (removed RemovedSetting) Error() string {
	return "the attachments setting " + removed.Name + " \"" + removed.Value + "\" was removed with the cloud option (ADR 0007): keep attachments in a local folder with the adapter \"local\""
}
