package protocol

import "time"

type Limits struct{}

func (Limits) MaxRecipients() int            { return 100 }
func (Limits) MaxLineLength() int            { return 1000 }
func (Limits) MaxErrors() int                { return 10 }
func (Limits) CommandTimeout() time.Duration { return 5 * time.Minute }
func (Limits) DataTimeout() time.Duration    { return 10 * time.Minute }
func (Limits) RefusalTimeout() time.Duration { return 10 * time.Second }
func (Limits) DefaultMaxSize() int64         { return 26214400 }
func (Limits) DefaultMaxConnections() int    { return 200 }
func (Limits) ReaderSize() int               { return 4096 }
func (Limits) LineEnding() string            { return "\r\n" }
func (Limits) BareLineEnding() string        { return "\n" }
