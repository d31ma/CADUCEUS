// MailActionTransition names one undo or redo: the status it leaves, the status it
// passes through and reaches, and the states it reads and writes.
type MailActionTransition struct {
	From, Intermediate, To, Source, Target string
	Now                                     int64
}
