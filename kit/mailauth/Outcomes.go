package mailauth

type Outcomes struct{}

func (Outcomes) Pass() string       { return "pass" }
func (Outcomes) Fail() string       { return "fail" }
func (Outcomes) SoftFail() string   { return "softfail" }
func (Outcomes) Neutral() string    { return "neutral" }
func (Outcomes) None() string       { return "none" }
func (Outcomes) Policy() string     { return "policy" }
func (Outcomes) TempError() string  { return "temperror" }
func (Outcomes) PermError() string  { return "permerror" }
func (Outcomes) Quarantine() string { return "quarantine" }
func (Outcomes) Reject() string     { return "reject" }
