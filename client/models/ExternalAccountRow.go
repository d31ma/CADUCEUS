type ExternalAccountRow struct {
	ID                 string              `json:"id"`
	MailboxAddress     string              `json:"mailboxAddress"`
	Provider           string              `json:"provider"`
	State              string              `json:"state"`
	Label              string              `json:"label"`
	Color              string              `json:"color"`
	CanPause           bool                `json:"canPause"`
	CanResume          bool                `json:"canResume"`
	CanReconnect       bool                `json:"canReconnect"`
	CanRetry           bool                `json:"canRetry"`
	CanRemove          bool                `json:"canRemove"`
	ProgressLabel      string              `json:"progressLabel"`
	StateLabel         string              `json:"stateLabel"`
	ReconnectUsername  string              `json:"reconnectUsername"`
	IsOAuth            bool                `json:"isOAuth"`
	IsSharedMailbox    bool                `json:"isSharedMailbox"`
	DelegateIdentity   string              `json:"delegateIdentity"`
	CanRead            bool                `json:"canRead"`
	CanSend            bool                `json:"canSend"`
	Reconnecting       bool                `json:"reconnecting"`
	ConfirmingRemoval  bool                `json:"confirmingRemoval"`
	ShowingDiagnostics bool                `json:"showingDiagnostics"`
	Diagnostics        *AccountDiagnostics `json:"diagnostics"`
	Mutating           bool                `json:"mutating"`
	Failure            *AccountFailure     `json:"failure"`
}
